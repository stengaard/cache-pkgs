all:
	GOARCH=amd64 GOOS=linux go build -o cache-pkgs.linux-amd64
	GOARCH=amd64 GOOS=darwin go build -o cache-pkgs.darwin-amd64
