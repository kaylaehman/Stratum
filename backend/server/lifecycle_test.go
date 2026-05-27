package server_test

import (
	"net/http"
	"testing"
)

func TestContainerLifecycleUnknown(t *testing.T) {
	srv, token := newNodeTestServer(t)
	c := &http.Client{}
	for _, action := range []string{"start", "stop", "restart"} {
		resp, err := c.Do(authReq(t, http.MethodPost, srv.URL+"/api/containers/nope/"+action, token, nil))
		if err != nil {
			t.Fatal(err)
		}
		resp.Body.Close()
		if resp.StatusCode != http.StatusNotFound {
			t.Errorf("%s unknown container = %d, want 404", action, resp.StatusCode)
		}
	}
}
