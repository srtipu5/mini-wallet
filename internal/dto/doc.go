// Package dto holds the data shapes that cross layer boundaries, one file per
// resource:
//
//   - *Request: HTTP request bodies, decoded by the handlers.
//   - *Input:   use-case inputs, passed from the handlers to the application.
//   - outputs:  use-case results that are not domain entities (e.g. HistoryPage).
//
// It depends only on the domain package.
package dto
