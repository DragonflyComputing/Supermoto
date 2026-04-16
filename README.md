# Supermoto
Tools for building websites with Go and PostgreSQL


## Install
```
go get github.com/Dragonfly-Computing/Supermoto@v1.0.0
```

## Usage
Supermoto is a small collection of Go functions for web development with html templates. It exists because I seem to copy these functions into every new project. It is intentionally minimal, no abstractions, no magic, no framework. Made to work with the standard library router.

If you are building something small and want to stay close to the standard library, it might be useful. If you are on a large team or need something feature-rich you should probably look elsewhere.


### database.go
Opens a pgx connection pool to PostgreSQL and verifies the connection with a ping.

```go
pool, err := supermoto.Connect(ctx, os.Getenv("DATABASE_URL"), nil)
```


### migrate.go
Runs forward-only SQL migrations from a directory. Migrations are tracked in a `schema_migrations` table and protected by a PostgreSQL advisory lock to prevent concurrent runs. Files must be named `NNN_description.sql` (e.g. `001_create_users.sql`).

```go
err := supermoto.Migrate(ctx, "./migrations", pool, nil)
```


### templates.go
Parses and serves Go HTML templates. The first path is the entry point; any additional paths are partials parsed into the same template set, so `{{template "name" .}}` calls resolve correctly. Pass `nil` as data if the template needs none.

```go
supermoto.Serve(w, map[string]any{"Name": "Robert Robertson"}, []string{"views/contract.html"}, nil)
```

To use a base layout with a content page, define `{{define "content"}}` in your page template and `{{template "content" .}}` in your base:

```go
supermoto.Serve(w, nil, []string{"views/base.html", "views/sites.html"}, nil)
```


## Notes
- Everything is written to return an error rather than making assumptions and continuing.
- All functions accept a `*log.Logger` as the last argument. Pass `nil` to use the default standard library logger.


## ToDo
- Middleware chain
- Session based authentication
- Example codebase? With recommended file structure?
