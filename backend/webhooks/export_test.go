package webhooks

// DisableURLValidationForTest turns off the post-time SSRF re-validation so
// tests can deliver to a local httptest server. Test-only — compiled only under
// `go test`; production code has no way to disable validation.
func (d *Dispatcher) DisableURLValidationForTest() { d.skipValidate = true }
