// A module of its own so that "go build ./..." in the root module does not
// compile the pipeline. "dagger develop" fills in the require entries here and
// generates internal/dagger and dagger.gen.go.
module dagger/schmetterpause

go 1.25.7
