// Sync files and directories to and from local and remote object stores
//
// Nick Craig-Wood <nick@craig-wood.com>
package main

import (
	"log"
	"crypto/x509"
	"crypto/tls"

	"cmd"
	_ "cmd/all" // import all commands
	_ "fs/all"  // import all fs
)

func main() {
	certs := x509.NewCertPool()
	pemData, err := ioutil.ReadFile("/etc/cacert.crt")
	if err == nil {
		certs.AppendCertsFromPEM(pemData)
		tls.Config.RootCAs = certs
	}
	if err := cmd.Root.Execute(); err != nil {
		log.Fatalf("Fatal error: %v", err)
	}
}
