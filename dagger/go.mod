// Eigenes Modul, damit "go build ./..." im Wurzelmodul die Pipeline nicht
// mitkompiliert. "dagger develop" ergaenzt hier die require-Eintraege und
// erzeugt internal/dagger sowie dagger.gen.go.
module dagger/schmetterpause

go 1.25.7
