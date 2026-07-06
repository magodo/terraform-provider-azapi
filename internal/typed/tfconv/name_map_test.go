package tfconv

import "testing"

func TestToCamelCase(t *testing.T) {
	t.Parallel()
	m := NewSnakeCamelNameMapper(nil)
	tests := map[string]string{
		"":                 "",
		"id":               "id",
		"some_field":       "someField",
		"some_field_name":  "someFieldName",
		"a_b_c":            "aBC",
		"http_url":         "httpUrl",
		"trailing_":        "trailing",
		"_leading":         "Leading",
		"already_snake_ok": "alreadySnakeOk",
	}
	for in, want := range tests {
		if got := m.ToCamelCase(in); got != want {
			t.Errorf("ToCamelCase(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestToSnakeCase(t *testing.T) {
	t.Parallel()
	m := NewSnakeCamelNameMapper(nil)
	tests := map[string]string{
		"":            "",
		"id":          "id",
		"someField":   "some_field",
		"SomeField":   "some_field",
		"HTTPServer":  "http_server",
		"URL":         "url",
		"userID":      "user_id",
		"parseXMLDoc": "parse_xml_doc",
		"a2b":         "a2b",
	}
	for in, want := range tests {
		if got := m.ToSnakeCase(in); got != want {
			t.Errorf("ToSnakeCase(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestSnakeCamelMapper_Overrides(t *testing.T) {
	t.Parallel()
	m := &SnakeCamelNameMapper{Overrides: map[string]string{
		"id":         "ID",           // acronym exception
		"custom_key": "totally_diff", // arbitrary mapping
	}}
	if got := m.ToAPI("id"); got != "ID" {
		t.Errorf("ToAPI id = %q", got)
	}
	if got := m.ToAPI("some_field"); got != "someField" {
		t.Errorf("ToAPI some_field = %q", got)
	}
	if got := m.ToTF("ID"); got != "id" {
		t.Errorf("ToTF ID = %q", got)
	}
	if got := m.ToTF("someField"); got != "some_field" {
		t.Errorf("ToTF someField = %q", got)
	}
	if got := m.ToTF("totally_diff"); got != "custom_key" {
		t.Errorf("ToTF totally_diff = %q", got)
	}
}
