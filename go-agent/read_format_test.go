package main

import (
	"encoding/xml"
	"testing"
)

func TestXMLReadRoundTrip(t *testing.T) {
	for _, content := range []string{"", "a < b && x > y\n", "literal ]]> </file> & <file>", "two ]]> ]]> endings"} {
		meta := map[string]any{"path": "a\"<&.ts", "offset": 201, "next_offset": 401, "total": 500}
		body := formatReadResult(meta, content, "xml")
		var result struct {
			Path    string `xml:"path,attr"`
			Offset  int    `xml:"offset,attr"`
			Next    int    `xml:"next_offset,attr"`
			Total   int    `xml:"total,attr"`
			Content string `xml:",chardata"`
		}
		if err := xml.Unmarshal([]byte(body), &result); err != nil {
			t.Fatal(err)
		}
		if result.Path != meta["path"] || result.Content != content || result.Offset != 201 || result.Next != 401 || result.Total != 500 {
			t.Fatalf("bad round trip: %+v", result)
		}
	}
}
