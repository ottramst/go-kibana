package kibana

// String is a helper routine that allocates a new string value
// to store v and returns a pointer to it
func String(v string) *string {
	p := new(string)
	*p = v
	return p
}

// Bool is a helper routine that allocates a new bool value
// to store v and returns a pointer to it.
func Bool(v bool) *bool {
	p := new(bool)
	*p = v
	return p
}
