package main

import (
	"crypto/tls"
	"encoding/json"
	"encoding/xml"
	"errors"
	"fmt"
	"github.com/creekorful/mvnparser"
	"github.com/spf13/cobra"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
)

const (
	outputStdout           = "stdout"
	outputGithub           = "github"
	defaultStarterMetadata = "https://start.spring.io"
	defaultBootMetadata    = "https://api.spring.io/projects/spring-boot/releases"
	defaultTypeID          = "maven-build"
	desc                   = `This command get the Spring version.

You can specify the '-b, --boot-version' flag to determine the Spring Boot version,
or you can leave it blank to use the current version.

  $ spring-version
  $ spring-version -b 3.1.2

You can also use the '-d, --dependency' flag multiple times to specify dependencies.
Alternatively, you can pass dependencies by separating them with commas, e.g. foo, bar.

  $ spring-version -d cloud-starter -d native
  $ spring-version -d cloud-starter,devtools -d native

You can use the '--starter-url' flag to define the URL of the starter metadata server,
and you can also utilize the '--boot-url' flag to establish the URL for the Spring Boot metadata server.

  $ spring-version --starter-url https://mystarter.com:8080
`
)

type Config struct {
	Metadata     Metadata
	BootVersion  string
	TypeID       string
	Dependencies []string
	Output       string
}

type Metadata struct {
	Starter  string
	Boot     string
	Insecure bool
}

type StarterMetadata struct {
	Type struct {
		Type    string `json:"type"`
		Default string `json:"default"`
		Values  []struct {
			ID          string `json:"id"`
			Name        string `json:"name"`
			Description string `json:"description"`
			Action      string `json:"action"`
			Tags        struct {
				Build   string `json:"build"`
				Dialect string `json:"dialect"`
				Format  string `json:"format"`
			} `json:"tags"`
		} `json:"values"`
	} `json:"type"`
}

type BootMetadata struct {
	Embedded struct {
		Releases []struct {
			Version         string `json:"version"`
			APIDocURL       string `json:"apiDocUrl"`
			ReferenceDocURL string `json:"referenceDocUrl"`
			Status          string `json:"status"`
			Current         bool   `json:"current"`
			Links           struct {
				Repository struct {
					Href string `json:"href"`
				} `json:"repository"`
				Self struct {
					Href string `json:"href"`
				} `json:"self"`
			} `json:"_links"`
		} `json:"releases"`
	} `json:"_embedded"`
}

func main() {
	c := Config{
		Metadata: Metadata{
			Starter: defaultStarterMetadata,
			Boot:    defaultBootMetadata,
		},
		TypeID: defaultTypeID,
		Output: outputStdout,
	}

	cmd := &cobra.Command{
		Use:          "spring-version",
		Short:        "Get Spring version",
		Long:         desc,
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			return run(c)
		},
	}

	flags := cmd.Flags()
	flags.StringVar(&c.Metadata.Starter, "starter-url", c.Metadata.Starter, "URL of Starter metadata")
	flags.StringVar(&c.Metadata.Boot, "boot-url", c.Metadata.Boot, "URL of Spring Boot metadata")
	flags.BoolVarP(&c.Metadata.Insecure, "insecure", "k", c.Metadata.Insecure, "Allow insecure metadata server connections when using SSL")
	flags.StringVar(&c.TypeID, "type-id", c.TypeID, "Type ID of the action in Spring Boot metadata")
	flags.StringVarP(&c.BootVersion, "boot-version", "b", c.BootVersion, "Spring Boot version")
	flags.StringSliceVarP(&c.Dependencies, "dependency", "d", c.Dependencies, "List of dependency identifiers to include in the generated project")
	flags.StringVarP(&c.Output, "output", "o", c.Output, "Output destination, where to write the result. Options: "+outputStdout+", "+outputGithub)
	if err := cmd.Execute(); err != nil {
		os.Exit(1)
	}
}

// run 進入點
func run(c Config) (err error) {
	fmt.Printf("Fetching Spring Boot Metadata from %s\n", c.Metadata.Boot)
	var boot BootMetadata
	if err = fromJson(c.Metadata.Boot, c.Metadata.Insecure, &boot); err != nil {
		return err
	}
	if c.BootVersion, err = boot.getBootVersion(c.BootVersion); err != nil {
		return err
	}

	fmt.Printf("Fetching Starter Metadata from %s\n", c.Metadata.Starter)
	var starter StarterMetadata
	if err = fromJson(c.Metadata.Starter, c.Metadata.Insecure, &starter); err != nil {
		return err
	}
	var action string
	if action, err = starter.getAction(c.TypeID); err != nil {
		return err
	}
	project, err := loadMavenProject(c.Metadata.Starter+action, c)
	if err != nil {
		return err
	}

	if err = writeln(c.Output, "spring-boot="+c.BootVersion); err != nil {
		return err
	}
	for k, v := range project.Properties {
		if strings.HasPrefix(k, "spring") {
			if err = writef(c.Output, "%s=%s\n", k, v); err != nil {
				return err
			}
		}
	}
	return nil
}

// fromJson 從傳入 Metadata 取得 json string, 並轉讀入 v
func fromJson(api string, insecure bool, v any) error {
	client := &http.Client{Transport: &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: insecure},
	}}
	response, err := client.Get(api)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	body, err := io.ReadAll(response.Body)
	if err != nil {
		return err
	}
	return json.Unmarshal(body, &v)
}

// contains 回傳 target 是否包含在 source 中
func contains(source []string, target string) bool {
	for _, v := range source {
		if v == target {
			return true
		}
	}
	return false
}

// getBootVersion 取得 spring boot version
func (b *BootMetadata) getBootVersion(target string) (string, error) {
	if target == "" {
		for _, r := range b.Embedded.Releases {
			if r.Current {
				target = r.Version
			}
		}
	}
	if target == "" {
		return "", errors.New("can not determine target version")
	}
	var versions []string
	for _, release := range b.Embedded.Releases {
		versions = append(versions, release.Version)
	}
	if !contains(versions, target) {
		return "", fmt.Errorf("spring-boot version '%s' is not listed in current supported versions: %s\n", target, strings.Join(versions, ", "))
	}
	return target, nil
}

// getAction 取得 starter 中對應 targetID 的 action
func (s *StarterMetadata) getAction(targetID string) (string, error) {
	for _, t := range s.Type.Values {
		if t.ID == targetID {
			return t.Action, nil
		}
	}
	return "", errors.New("can not determine type action")
}

// loadMavenProject 從 starter 載入 maven project
func loadMavenProject(apiURL string, c Config) (project *mvnparser.MavenProject, err error) {
	queryParams := url.Values{
		"BootVersion":  []string{c.BootVersion},
		"dependencies": c.flattenedDependencies(),
	}
	apiURL += "?" + queryParams.Encode()
	response, err := http.Get(apiURL)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	body, err := io.ReadAll(response.Body)
	if err != nil {
		return nil, err
	}
	err = xml.Unmarshal(body, &project)
	return project, err
}

// flattenedDependencies 將 dependencies 中包含逗號的打平
func (c *Config) flattenedDependencies() (dependencies []string) {
	for _, item := range c.Dependencies {
		dependencies = append(dependencies, strings.Split(item, ",")...)
	}
	return dependencies
}

// writeln 將 text 寫入 output 並且斷行
func writeln(output, text string) error {
	return writef(output, "%s\n", text)
}

// writef 將 text 格式化後, 寫入 output
func writef(output, format string, a ...any) error {
	return write(output, fmt.Sprintf(format, a...))
}

// writeln 將 text 寫入 output
func write(output, text string) (err error) {
	var out *os.File
	switch output {
	case outputStdout:
		out = os.Stdout
	case outputGithub:
		path := os.Getenv("GITHUB_OUTPUT")
		if path == "" {
			return fmt.Errorf("environment variable GITHUB_OUTPUT must be set")
		}
		out, err = os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		if err != nil {
			return fmt.Errorf("could not open github output file for writing: %w", err)
		}
		defer out.Close()
	default:
		return fmt.Errorf("unsupported output type to new writer: %s", output)
	}
	_, err = out.WriteString(text)
	return err
}
