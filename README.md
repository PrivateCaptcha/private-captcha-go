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
// ... handle err

output, err := client.Verify(ctx, pc.VerifyInput{Solution: solution, Retry: true})
// ... handle err

if !output.Success {
	fmt.Printf("Captcha verification failed. Error: %s", result.Error())
}
```
