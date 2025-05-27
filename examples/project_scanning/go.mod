module project-scanning-example

go 1.24.3

replace github.com/b-open-io/agent-master-engine => ../../

require github.com/b-open-io/agent-master-engine v0.0.0-00010101000000-000000000000

require (
	github.com/fsnotify/fsnotify v1.7.0 // indirect
	golang.org/x/sys v0.33.0 // indirect
)
