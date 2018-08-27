source ./build-plugin.sh $1
echo build main
CGO_ENABLED=1 go build $1 main.go

