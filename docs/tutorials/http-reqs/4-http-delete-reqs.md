# DELETE Requests 

```
const (
    USER1_ENDPOINT = https://jsonplaceholder.typicode.com/users/1
)

# Note: https://jsonplaceholder.typicode.com provides a mocked API,
# the requests we make further in the code have no real effects.

manifest {
    permissions: {
        # allow making DELETE requests to the specified endpoint.
        delete: USER1_ENDPOINT
    }
}

# Make a DELETE request to delete the user of id 1
resp = http.delete!(USER1_ENDPOINT)

print(resp)
```