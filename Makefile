package: package.zip

package.zip: app host.json hello/function.json process/function.json webhook/function.json
	zip -r $@ $^

app: main.go go.mod go.sum
	env CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -a -o $@

clean:
	$(RM) package.zip app

.PHONY: package clean