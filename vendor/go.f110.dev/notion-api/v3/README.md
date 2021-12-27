notion-api
---

[![Go Reference](https://pkg.go.dev/badge/go.f110.dev/notion-api.svg)](https://pkg.go.dev/go.f110.dev/notion-api)

API client for Notion written by Go.

Currently under active development. All APIs will be changed possibly.

# Usage

First, import from your code.

```go
import "go.f110.dev/notion-api"
```

and you also need `golang.org/x/oauth2` module for *http.Client.

```go
import "golang.org/x/oauth2"
```

After import, you can create the client with *http.Client.

```go
ts := oauth2.StaticTokenSource(&oauth2.Token{AccessToken: token})
tc := oauth2.NewClient(context.Background(), ts)
client, err := notion.New(tc, notion.BaseURL)
```

And example code exists under [example directory](./example)

# Supported methods

* [x] [Retrieve a database](https://developers.notion.com/reference/get-database)
* [x] [Query a database](https://developers.notion.com/reference/post-database-query)
* [x] [List databases](https://developers.notion.com/reference/get-databases)
* [x] [Retrieve a user](https://developers.notion.com/reference/get-user)
* [x] [List all users](https://developers.notion.com/reference/get-users)
* [x] [Retrieve a page](https://developers.notion.com/reference/get-page)
* [x] [Create a page](https://developers.notion.com/reference/post-page)
* [x] [Update page properties](https://developers.notion.com/reference/patch-page)
* [x] [Retrieve block children](https://developers.notion.com/reference/get-block-children)
* [x] [Append block children](https://developers.notion.com/reference/patch-block-children)
* [x] [Search](https://developers.notion.com/reference/post-search)
* [x] [Create a database](https://developers.notion.com/reference/create-a-database)
* [x] [Retrieve a block](https://developers.notion.com/reference/retrieve-a-block)
* [x] [Update a block](https://developers.notion.com/reference/update-a-block)
* [x] [Update a database](https://developers.notion.com/reference/update-a-database)
* [x] [Delete a block](https://developers.notion.com/reference/delete-a-block)
* [x] [Retrieve a page property](https://developers.notion.com/reference/retrieve-a-page-property)

# Implemented version

Implemented [Notion API version](https://developers.notion.com/reference/versioning) is **2021-08-16** .

# Author

Fumihiro Ito