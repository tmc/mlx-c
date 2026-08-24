package customspec

import (
	"bytes"
	"fmt"
	"strings"
)

// RenderHeader renders the custom C header described by spec.
func RenderHeader(spec Spec) ([]byte, error) {
	if err := spec.validate(); err != nil {
		return nil, err
	}
	if !spec.Generate.Header {
		return nil, fmt.Errorf("%s: header generation is not enabled", spec.Name)
	}
	var b bytes.Buffer
	fmt.Fprintf(&b, "/* %s */\n", spec.Copyright)
	fmt.Fprintln(&b, "/* This file is auto-generated. Do not edit manually. */")
	fmt.Fprintln(&b)
	fmt.Fprintf(&b, "#ifndef %s\n", spec.IncludeGuard)
	fmt.Fprintf(&b, "#define %s\n\n", spec.IncludeGuard)
	for _, inc := range spec.Includes {
		fmt.Fprintf(&b, "#include <%s>\n", inc)
	}
	if len(spec.Includes) > 0 {
		fmt.Fprintln(&b)
	}
	fmt.Fprintln(&b, "#ifdef __cplusplus")
	fmt.Fprintln(&b, "extern \"C\" {")
	fmt.Fprintln(&b, "#endif")
	fmt.Fprintln(&b)
	renderDoc(&b, []string{
		"\\defgroup " + spec.Group.Name + " " + spec.Group.Title,
		spec.Group.Doc,
	})
	fmt.Fprintln(&b, "/**@{*/")
	fmt.Fprintln(&b)
	for i, item := range spec.Items {
		if i > 0 {
			fmt.Fprintln(&b)
		}
		renderDoc(&b, strings.Split(item.Doc, "\n"))
		switch item.Kind {
		case "struct":
			if !item.Opaque {
				return nil, fmt.Errorf("%s: struct %s is not opaque", spec.Name, item.Name)
			}
			fmt.Fprintf(&b, "typedef struct %s_ { void* ctx; } %s;\n", item.Name, item.Name)
		case "enum":
			fmt.Fprintf(&b, "typedef enum %s_ {\n", item.Name)
			prev := 0
			for j, value := range item.Values {
				if j == 0 || value.Value != prev+1 {
					fmt.Fprintf(&b, "  %s = %d,\n", value.Name, value.Value)
				} else {
					fmt.Fprintf(&b, "  %s,\n", value.Name)
				}
				prev = value.Value
			}
			fmt.Fprintf(&b, "} %s;\n", item.Name)
		case "function":
			fmt.Fprintf(&b, "%s;\n", item.Signature)
		default:
			return nil, fmt.Errorf("%s: cannot render %s %s", spec.Name, item.Kind, item.Name)
		}
	}
	fmt.Fprintln(&b)
	fmt.Fprintln(&b, "/**@}*/")
	fmt.Fprintln(&b)
	fmt.Fprintln(&b, "#ifdef __cplusplus")
	fmt.Fprintln(&b, "}")
	fmt.Fprintln(&b, "#endif")
	fmt.Fprintln(&b)
	fmt.Fprintln(&b, "#endif")
	return b.Bytes(), nil
}

func renderDoc(b *bytes.Buffer, lines []string) {
	for len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	fmt.Fprintln(b, "/**")
	for _, line := range lines {
		if line == "" {
			fmt.Fprintln(b, " *")
			continue
		}
		fmt.Fprintf(b, " * %s\n", line)
	}
	fmt.Fprintln(b, " */")
}
