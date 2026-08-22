# Control Room Asset Embed Specification

## Purpose

Zero-build delivery of HTML, CSS, and vanilla ES modules from the serve binary.

## Requirements

### Requirement: Zero-Build Embedded ES Module Delivery

The binary MUST embed HTML, CSS, and vanilla ES module assets and serve them over HTTP with Content-Type `application/javascript`, `text/css`, or `text/html` as appropriate. Assets MUST run without Node.js, npm, or a bundler.

#### Scenario: Static assets served with matching MIME type

- GIVEN assets embedded in the binary
- WHEN requesting GET /shell.js or GET /style.css
- THEN the server MUST return HTTP 200 with application/javascript or text/css

#### Scenario: Module import resolution

- GIVEN modular ES modules embedded in the binary
- WHEN the browser fetches an imported module GET /store.js
- THEN the server MUST return HTTP 200 with application/javascript

#### Scenario: Missing asset returns 404

- GIVEN a running server
- WHEN requesting GET /missing.js
- THEN the server MUST return HTTP 404 Not Found
