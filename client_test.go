package privatecaptcha

import (
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"net/http"
	"os"
	"testing"
)

const (
	solutionsCount = 16
	solutionLength = 8
)

func fetchTestPuzzle() ([]byte, error) {
	req, err := http.NewRequest(http.MethodGet, "https://api.privatecaptcha.com/puzzle?sitekey=aaaaaaaabbbbccccddddeeeeeeeeeeee", nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Origin", "not.empty")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}

	defer resp.Body.Close()

	return io.ReadAll(resp.Body)
}

func TestStubPuzzle(t *testing.T) {
	t.Parallel()

	puzzle, err := fetchTestPuzzle()
	fmt.Println(string(puzzle))
	if err != nil {
		t.Fatal(err)
	}

	client, err := NewClient(Configuration{
		APIKey: os.Getenv("PC_API_KEY"),
	})
	if err != nil {
		t.Fatal(err)
	}

	emptySolutionsBytes := make([]byte, solutionsCount*solutionLength)
	solutionsStr := base64.StdEncoding.EncodeToString(emptySolutionsBytes)
	payload := fmt.Sprintf("%s.%s", solutionsStr, string(puzzle))

	output, err := client.Verify(context.TODO(), VerifyInput{Solution: payload})
	if err != nil {
		t.Fatal(err)
	}

	if !output.Success || (output.Code != TestPropertyError) {
		t.Errorf("Unexpected result (%v) or error (%v)", output.Success, output.Code)
	}
}

func TestVerifyError(t *testing.T) {
	t.Parallel()

	puzzle, err := fetchTestPuzzle()
	fmt.Println(string(puzzle))
	if err != nil {
		t.Fatal(err)
	}

	client, err := NewClient(Configuration{
		APIKey: os.Getenv("PC_API_KEY"),
	})
	if err != nil {
		t.Fatal(err)
	}

	emptySolutionsBytes := make([]byte, solutionsCount*solutionLength/2)
	solutionsStr := base64.StdEncoding.EncodeToString(emptySolutionsBytes)
	payload := fmt.Sprintf("%s.%s", solutionsStr, string(puzzle))

	output, err := client.Verify(context.TODO(), VerifyInput{Solution: payload})
	if err != nil {
		t.Fatal(err)
	}

	if output.Success || (output.Code != ParseResponseError) {
		t.Errorf("Unexpected result (%v) or error (%v)", output.Success, output.Code)
	}
}
