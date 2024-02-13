# Filesystem Routing Basics 

```
const (
    HOST = https://localhost:8080
)

manifest {
    permissions: {
        provide: HOST

        read: %/...

        # allow persisting the self signed certificate that is automatically generated.
        write: %/.dev/self_signed*
    }
}


server = http.Server!(HOST, {
    routing: {
        # Directory for static resources such as CSS and JS files.
        static: /.tutorial-files/static/

        # Directory containing handler modules (Inox files). Explore it to see how routing works.
        dynamic: /.tutorial-files/fs-routing-basics/
    }
})

server.wait_closed()
```