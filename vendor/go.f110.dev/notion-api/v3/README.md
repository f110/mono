notion-api
---

[![Go Reference](https://pkg.go.dev/badge/go.f110.dev/notion-api.svg)](https://pkg.go.dev/go.f110.dev/notion-api)

API client for Notion written by Go.

Currently under active development. All APIs will be changed possibly.

# Usage

First, import from your code.

```go
import "go.f110.dev/notion-api/v3"
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

* Database
    * [x] [Retrieve a database](https://developers.notion.com/reference/get-database)
    * [x] [Query a database](https://developers.notion.com/reference/post-database-query)
    * [x] [Create a database](https://developers.notion.com/reference/create-a-database)
    * [x] [Update a database](https://developers.notion.com/reference/update-a-database)
* User
    * [x] [Retrieve a user](https://developers.notion.com/reference/get-user)
    * [x] [List all users](https://developers.notion.com/reference/get-users)
    * [ ] [Retrieve your token's bot user](https://developers.notion.com/reference/update-property-schema-object) 
* Page
    * [x] [Retrieve a page](https://developers.notion.com/reference/get-page)
    * [x] [Create a page](https://developers.notion.com/reference/post-page)
    * [x] [Update page properties](https://developers.notion.com/reference/patch-page)
    * [x] [Retrieve a page property item](https://developers.notion.com/reference/retrieve-a-page-property)
* Block
    * [x] [Retrieve block children](https://developers.notion.com/reference/get-block-children)
    * [x] [Append block children](https://developers.notion.com/reference/patch-block-children)
    * [x] [Retrieve a block](https://developers.notion.com/reference/retrieve-a-block)
    * [x] [Delete a block](https://developers.notion.com/reference/delete-a-block)
    * [x] [Update a block](https://developers.notion.com/reference/update-a-block)
* Search
    * [x] [Search by title](https://developers.notion.com/reference/post-search)
* Comment
    * [ ] [Create comment](https://developers.notion.com/reference/create-a-comment)
    * [ ] [Retrieve Comments](https://developers.notion.com/reference/retrieve-a-comment)

# Implemented version

Implemented [Notion API version](https://developers.notion.com/reference/versioning) is **2022-06-28** .

# Author

Fumihiro Ito
