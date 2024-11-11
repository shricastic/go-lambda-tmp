# Go Lambda Handler for AWS 

1. How to compile binary for arm64 linux aws machine 

```GOOS=linux GOARCH=arm64 go build -o main.go``` 

2. Zip binary up and upload to aws lambda function 

``` zip lambda-handler.zip bootstrap ```

first part lil hard to remember. thats it i guess!

