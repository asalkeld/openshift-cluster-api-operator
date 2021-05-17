package baremetal

//go:generate go run github.com/go-bindata/go-bindata/go-bindata/ -nometadata -pkg $GOPACKAGE -ignore=bindata.go -ignore=generate.go  ./...
//go:generate gofmt -s -l -w bindata.go
