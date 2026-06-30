// Copyright (c) the go-ruby-sinatra/sinatra authors
//
// SPDX-License-Identifier: BSD-3-Clause

package sinatra

import "strings"

// mimeTypes maps a Sinatra mime symbol (or file extension) to its media type,
// covering the entries Sinatra registers via Rack::Mime that callers reach for
// with content_type :sym. Lookups are by the bare name (e.g. "json") or by a
// leading-dot extension (".json").
var mimeTypes = map[string]string{
	"css":   "text/css",
	"csv":   "text/csv",
	"htm":   "text/html",
	"html":  "text/html",
	"js":    "text/javascript",
	"json":  "application/json",
	"txt":   "text/plain",
	"text":  "text/plain",
	"xml":   "application/xml",
	"xhtml": "application/xhtml+xml",
	"atom":  "application/atom+xml",
	"rss":   "application/rss+xml",
	"yaml":  "text/yaml",
	"yml":   "text/yaml",
	"pdf":   "application/pdf",
	"zip":   "application/zip",
	"gz":    "application/gzip",
	"png":   "image/png",
	"jpg":   "image/jpeg",
	"jpeg":  "image/jpeg",
	"gif":   "image/gif",
	"svg":   "image/svg+xml",
	"ico":   "image/vnd.microsoft.icon",
	"webp":  "image/webp",
	"bin":   "application/octet-stream",
}

// MimeType resolves a Sinatra mime symbol or extension to a media type, like
// Sinatra::Base.mime_type. A value already containing a "/" is returned as is
// (Sinatra treats a full media type as itself). It returns "" when unknown.
func MimeType(sym string) string {
	if sym == "" {
		return ""
	}
	if strings.Contains(sym, "/") {
		return sym
	}
	key := strings.ToLower(strings.TrimPrefix(sym, "."))
	return mimeTypes[key]
}

// addCharsetExact lists the non-text media types Sinatra augments with a
// charset (settings.add_charset); text/* is handled separately by prefix.
var addCharsetExact = map[string]bool{
	"application/javascript": true,
	"application/xml":        true,
	"application/xhtml+xml":  true,
}

// wantsCharset reports whether Sinatra appends ;charset to mediaType, matching
// settings.add_charset (the application/* exceptions and the /^text\// regexp).
func wantsCharset(mediaType string) bool {
	if strings.HasPrefix(mediaType, "text/") {
		return true
	}
	return addCharsetExact[mediaType]
}

// buildContentType resolves a content_type argument to the final header value,
// mirroring Sinatra#content_type: it looks up the media type, appends
// ;charset=<encoding> when add_charset matches and no charset is already
// present, and returns ("", false) for an unknown type so the caller can raise.
func buildContentType(typeArg, encoding string, explicitCharset string) (string, bool) {
	mt := MimeType(typeArg)
	if mt == "" {
		return "", false
	}
	hasCharset := strings.Contains(strings.ToLower(mt), "charset")
	switch {
	case hasCharset:
		// Already carries a charset; leave it.
	case explicitCharset != "":
		mt += ";charset=" + explicitCharset
	case wantsCharset(mt):
		mt += ";charset=" + encoding
	}
	return mt, true
}
