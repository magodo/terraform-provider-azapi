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
		"SomeField":   "_some_field",
		"HTTPServer":  "_h_t_t_p_server",
		"URL":         "_u_r_l",
		"userID":      "user_i_d",
		"parseXMLDoc": "parse_x_m_l_doc",
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
	m := NewSnakeCamelNameMapper(map[string]string{
		"id":         "ID",
		"custom_key": "CustomKey",
	})

	tests := map[string]string{
		"":           "",
		"ID":         "id",
		"CustomKey":  "custom_key",
		"HTTPServer": "_h_t_t_p_server",
	}
	for in, want := range tests {
		if got := m.ToSnakeCase(in); got != want {
			t.Errorf("ToSnakeCase(%q) = %q, want %q", in, got, want)
		}
	}
}
