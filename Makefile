package: package.zip

package.zip: app host.json hello/function.json process/function.json webhook/function.json
	mkdir -p package/{,hello,process,webhook}
	cp app host.json package
	cp hello/function.json package/hello
	cp process/function.json package/process
	cp webhook/function.json package/webhook
	(cd package && zip -r ../$@ .)

app: main.go go.mod go.sum
	env CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -a -o $@
