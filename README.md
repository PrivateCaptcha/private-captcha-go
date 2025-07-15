# Official Go client for Private Captcha API

![GitHub go.mod Go version](https://img.shields.io/github/go-mod/go-version/PrivateCaptcha/private-captcha-go) ![CI](https://github.com/PrivateCaptcha/private-captcha-go/actions/workflows/ci.yaml/badge.svg)

## Usage

Install this library:

```bash
go get github.com/PrivateCaptcha/private-captcha-go
```

Add import:

```go
import pc "github.com/PrivateCaptcha/private-captcha-go"
```

Use client:

```go
client, err := pc.NewClient(pc.Configuration{APIKey: "pc_abcdef"})

result, err := client.Verify(ctx, solution)
if !result.Success {
	fmt.Printf("Captcha verification failed. Error: %s", result.Error())
}
```
