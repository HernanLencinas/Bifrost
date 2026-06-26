package main

// Version se define en tiempo de compilación con -ldflags.
// En desarrollo (go run sin flags) el valor por defecto es "dev".
var Version = "dev"

func getAppVersion() string {
	return Version
}
