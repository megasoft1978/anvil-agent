package main

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"strings"
)

func formatReadResult(read map[string]any, content, format string) string {
	if format != "xml" {
		return fmt.Sprintf("File: %v\nOffset: %v; next_offset: %v; total: %v\n\n%s", read["path"], read["offset"], read["next_offset"], read["total"], content)
	}
	var path bytes.Buffer
	_ = xml.EscapeText(&path, []byte(fmt.Sprint(read["path"])))
	// Split CDATA terminators so arbitrary source remains valid XML and round-trips unchanged.
	content = strings.ReplaceAll(content, "]]>", "]]]]><![CDATA[>")
	return fmt.Sprintf("<file path=\"%s\" offset=\"%v\" next_offset=\"%v\" total=\"%v\"><![CDATA[%s]]></file>", path.String(), read["offset"], read["next_offset"], read["total"], content)
}
