# First stage: build the executable
FROM golang:1.11-alpine AS build

# Create the user and group files that will be used in the running container to
# run the process an unprivileged user
RUN mkdir /user && \
    echo 'nobody:x:65534:65534:nobody:/:' > /user/passwd && \
    echo 'nobody:x:65534:' > /user/group

# Install the CA certificates for the app to be able to make calls to HTTPS endpoints
RUN apk add --no-cache ca-certificates git

# Set the environment variables for the go command:
# - CGO_ENABLED=0 to build a statically-linked executable
ENV CGO_ENABLED=0

# Set the working directory outside $GOPATH to enable the support for modules
WORKDIR /src

# Import the code from the context
COPY ./ ./

# Build the statically linked executable
# TODO: modules caching?
RUN sh /src/build/docker.sh

# Final stage: the running container
FROM scratch AS final

# Import the user and group files from the first stage
COPY --from=build /user/group /user/passwd /etc/

# Import the CA certificates for enabling HTTPS
COPY --from=build /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/

# Import the compiled executable and default config file
COPY --from=build /server /server
COPY --from=build /src/config/deployer.example.yml /etc/nomad-deployer/deployer.yml

# Declare the port on which the server will be exposed
EXPOSE 8000

# Perform any further action as an unprivileged user
USER nobody:nobody

# Run the compiled binary
ENTRYPOINT ["/server"]
