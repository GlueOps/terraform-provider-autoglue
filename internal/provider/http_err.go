package provider

import (
	"fmt"
	"io"
	"net/http"
)

func httpErr(err error, resp *http.Response) string {
	if resp == nil {
		return err.Error()
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	return fmt.Sprintf("status=%d: %s (body=%s)", resp.StatusCode, resp.Status, string(b))
}

func isNotFound(resp *http.Response) bool {
	return resp != nil && resp.StatusCode == http.StatusNotFound
}
