package s3store

import "testing"

func TestParseEndpoint(t *testing.T) {
	ep, ssl := parseEndpoint("https://s3.amazonaws.com", false)
	if ep != "s3.amazonaws.com" || !ssl {
		t.Fatalf("https: %s %v", ep, ssl)
	}
	ep, ssl = parseEndpoint("http://minio:9000", true)
	if ep != "minio:9000" || ssl {
		t.Fatalf("http: %s %v", ep, ssl)
	}
	ep, ssl = parseEndpoint("minio:9000", false)
	if ep != "minio:9000" || ssl {
		t.Fatalf("bare: %s %v", ep, ssl)
	}
}

func TestContentType(t *testing.T) {
	if contentType("thumbs/a.png") != "image/png" {
		t.Fatal(contentType("thumbs/a.png"))
	}
	if contentType("outputs/a.zip") != "application/zip" {
		t.Fatal(contentType("outputs/a.zip"))
	}
}
