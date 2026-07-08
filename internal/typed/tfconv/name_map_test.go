package tfconv

import "testing"

func TestToCamelCaseNaive(t *testing.T) {
	t.Parallel()
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
		if got := ToCamelCaseNaive(in); got != want {
			t.Errorf("ToCamelCaseNaive(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestToSnakeCaseNaive(t *testing.T) {
	t.Parallel()
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
		if got := ToSnakeCaseNaive(in); got != want {
			t.Errorf("ToSnakeCaseNaive(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestToSnakeCaseSmart(t *testing.T) {
	t.Parallel()
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
		if got := ToSnakeCaseSmart(in); got != want {
			t.Errorf("ToSnakeCaseSmart(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestCamelSnakeMapper_ToSnakeCase(t *testing.T) {
	t.Parallel()
	m := NewCamelSnakeNameMapper(map[string]string{
		"aID":         "a_id",
		"aID.bId.aid": "a_id",
	})

	tests := []struct {
		in   string
		want string
	}{
		{"", ""},
		{"id", "id"},
		{"aID", "a_id"},
		{"aID.bId", "b_id"},
		{"aID.bId.aid", "a_id"},
	}
	for _, tt := range tests {
		if got := m.ToSnakeCase(tt.in); got != tt.want {
			t.Errorf("CamelSnakeMapper.ToSnakeCase(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestCamelSnakeMapper_ToCamelCase(t *testing.T) {
	t.Parallel()
	m := NewCamelSnakeNameMapper(map[string]string{
		"aID":         "a_id",
		"aID.bId.aid": "a_id",
	})

	tests := []struct {
		in   string
		want string
	}{
		{"", ""},
		{"a_id", "aID"},
		{"a_id.b_id", "bId"},
		{"a_id.b_id.aid", "aid"},
	}
	for _, tt := range tests {
		if got := m.ToCamelCase(tt.in); got != tt.want {
			t.Errorf("CamelSnakeMapper.ToCamelCase(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}
