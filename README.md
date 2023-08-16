# Get Spring Version in Go

[![Go Report Cart](https://goreportcard.com/badge/github.com/shihyuho/go-spring-version)](https://goreportcard.com/report/github.com/shihyuho/go-spring-version)

Get Spring Version, written in Go

## Usage

### Input Variables

| Name | Description |
|------|-------------|
| boot-url | URL of Spring Boot metadata (default `https://api.spring.io/projects/spring-boot/releases`) |
| starter-url | URL of Starter metadata (default `https://start.spring.io`) |
| insecure | `true/false`, Allow insecure metadata server connections when using SSL (default `false`) |
| boot-version | Spring Boot version, e.g. `x.y.z` (default: current version) |
| dependencies | List of dependency identifiers to include in the generated project, can separate with commas, e.g., `cloud-starter`. |

### Output

This action output `spring-{project}`, For example when you fetch current spring-boot with `cloud-starter` dependency, it will give you the following outputs:

```
spring-boot: 3.1.4
spring-cloud: 2022.0.4
```

### Example

```yaml
jobs:
  spring-version:
    runs-on: ubuntu-latest
    steps:
      - id: get-spring-version
        uses: shihyuho/go-spring-version@v1
        with:
          dependencies: "cloud-starter"
      - run: 'echo spring-boot: ${{ steps.get-spring-version.outputs.spring-boot }}'
      - run: 'echo spring-cloud: ${{ steps.get-spring-version.outputs.spring-cloud }}'
```

> See also: [Access context information in workflows and actions](https://docs.github.com/en/actions/learn-github-actions/contexts)
