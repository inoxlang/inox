
# Builtin

- [Errors](#errors)
- [Resource Manipulation](#resource-manipulation)
- [Chrome](#chrome)
- [Containers](#containers)
- [Crypto](#crypto)
- [Dns](#dns)
- [Fs](#fs)
- [Functional](#functional)
- [Html](#html)
- [Http](#http)
- [Rand](#rand)
- [Tcp](#tcp)
## Errors

### Error

the Error function creates an error from the provided string and an optional data argument.

**examples**

```inox
Error("failed to create user")
```
```inox
Error("failed to create user", {user_id: 100})
```

## Resource Manipulation

### read

read is a general purpose function that reads the content of a file, a directory or a HTTP resource. The content is parsed by default, to disable parsing use --raw after the resource's name: a byte slice  will be returned instead.

**examples**

```inox
read ./
# output: 
[
  dir/
  file.txt 1kB 
]

```
```inox
read ./file.txt
# output: 
hello
```
```inox
read ./file.json
# output: 
{"key": "value"}
```
```inox
read https://jsonplaceholder.typicode.com/posts/1
# output: 
{
  "body": "quia et suscipit\nsuscipit recusandae consequuntur expedita....", 
  "id": 1.0, 
  "title": "sunt aut facere repellat provident occaecati excepturi optio reprehenderit", 
  "userId": 1.0
}

```
### create

create is a general purpose function that can create a file, a directory or a HTTP resource.

**examples**

```inox
create ./dir/
```
```inox
create ./empty-file.txt
```
```inox
create ./file.txt "content"
```
### update

update is a general purpose function that updates an existing resource, it has 2 modes: append and replace. Replace is the default mode.

**examples**

```inox
update ./file.txt append 'additional content'`
```
```inox
update ./file.txt 'new content'`
```
```inox
update ./file.txt replace 'new content'`
```
```inox
update <url> tojson({})'
```
### delete

delete is a general purpose function that deletes a resource, deletion is recursive for directories.

**examples**

```inox
delete ./file.txt
```
```inox
delete ./dir/
```
```inox
delete <url>
```

## Chrome

### chrome

chrome namespace
### chrome.Handle

the Handle function creates a new Chrome handle

**examples**

```inox
chrome.Handle!()
```

## Containers

### Graph

the Graph function creates a directed Graph
### Tree

the Tree function creates a tree from a udata value

**examples**

```inox
Tree(udata "root")
```
### Stack

the Stack function creates a stack from an iterable

**examples**

```inox
Stack([])
```
```inox
Stack([1])
```
### Queue

the Queue function creates a queue from an iterable

**examples**

```inox
Queue([])
```
```inox
Queue([1])
```
### Set

the Set function creates a set from an iterable, only representable (serializable) values are allowed

**examples**

```inox
Set([])
```
```inox
Set([1])
```
### Map

the Map function creates a map from a list of flat entries

**examples**

```inox
Map(["key1", 10, "key2", 20]
```
### Ranking

the Ranking function creates a ranking from a list of flat entries. An entry is composed of a value and a floating-point score.  The value with the highest score has the first rank (0), values with the same score have the same rank.

**examples**

```inox
Ranking(["best player", 10.0, "other player", 5.0])
```
```inox
Ranking(["best player", 10.0, "other player", 10.0])
```
### Thread

the Thread function creates a thread from an iterable.

**examples**

```inox
Thread([{message: "hello", author-id: "5958"}])
```

## Crypto

### hash_password

the hash_password function hashes a password string using the Bcrypt hashing algorithm, it accepts the cost   as a second optional argument. The cost is an integer between 4 and 31, it defaults to 7.

**examples**

```inox
hash_password("password")
# output: 
"JDJhJDA3JHNNNzRwaFVNUFVCNzVDQmxINU5HaWVOZERKZ09IRkx4a2xxYTFPTktsV3Nkd2JqampmYmxT"
```
```inox
hash_password("password", 10)
# output: 
"JDJhJDEwJGhLODFiVThNdTlJZXVRMXVZdHlIUi5oOS5GSXljNWpYWGcwaVhXWUZYZC5YRTduR1hmSjl1"
```
### check_password

the check_password hashes a string or a byte sequences using the SHA-256 algorithm

**examples**

```inox
check_password("password", "JDJhJDA3JHNNNzRwaFVNUFVCNzVDQmxINU5HaWVOZERKZ09IRkx4a2xxYTFPTktsV3Nkd2JqampmYmxT")
# output: 
true
```
### sha256

the sha256 function hashes a string or a byte sequence with the SHA-256 algorithm

**examples**

```inox
sha256("string")
# output: 
0x[473287f8298dba7163a897908958f7c0eae733e25d2e027992ea2edc9bed2fa8]
```
### sha384

the sha384 function hashes a string or a byte sequence with the SHA-384 algorithm

**examples**

```inox
sha384("string")
# output: 
0x[36396a7e4de3fa1c2156ad291350adf507d11a8f8be8b124a028c5db40785803ca35a7fc97a6748d85b253babab7953e]
```
### sha512

the sha512 function hashes a string or a byte sequence with the SHA-512 algorithm

**examples**

```inox
sha512("string")
# output: 
0x[2757cb3cafc39af451abb2697be79b4ab61d63d74d85b0418629de8c26811b529f3f3780d0150063ff55a2beee74c4ec102a2a2731a1f1f7f10d473ad18a6a87]
```

## Dns

### dns.resolve

the dns.resolve function retrieves DNS records of the given type.

**examples**

```inox
dns.resolve!("github.com" "A")
```

## Fs

### fs.mkfile

The fs.mkfile function takes a file path as first argument. It accepts a --readonly switch that causes  the created file to not have the write permission ; and a %readable argument that is the content of the file to create.

**examples**

```inox
fs.mkfile ./file.txt
```
```inox
fs.mkfile ./file.txt "content"
```
### fs.mkdir

the fs.mkdir function takes a directory path as first argument & and optional dictionary literal as a second argument. The second argument recursively describes the content of the directory.

**examples**

```inox
fs.mkdir ./dir_a/
```
```inox
dir_content = :{
    ./subdir_1/: [./empty_file]
    ./subdir_2/: :{  
        ./file: "foo"
    }
    ./file: "bar"
}

fs.mkdir ./dir_b/ $dir_content


```
### fs.read

the fs.read function behaves exactly like the read function but only works on files & directories.
### fs.read_file

the fs.read function behaves exactly like the read function but only works on files.
### fs.ls

the fs.ls function takes a directory path or a path pattern as first argument and returns a list of entries, if no argument is provided the ./ directory is used.

**examples**

```inox
fs.ls()
```
```inox
fs.ls ./
```
```inox
fs.ls %./*.json
```
### fs.rename

the fs.rename (fs.mv) function renames a file, it takes two path arguments.  An error is returned if a file already exists at the target path.
### fs.cp

the fs.cp function copies a file/dir at a destination or a list of files in a destination directory, the copy is recursive by default. As you can see this behaviour is not exactly the same as the cp command on Unix. An error is returned if a file or a directory already exists at one of the target paths (recursive).

**examples**

```inox
fs.cp ./file.txt ./file_copy.txt
```
```inox
fs.cp ./dir/ ./dir_copy/
```
```inox
fs.cp [./file.txt, ./dir/] ./dest_dir/
```
### fs.exists

the fs.exists takes a path as first argument and returns a boolean
### fs.isdir

the fs.isdir function returns true if there is a directory at the given path
### fs.isfile

the fs.isfile returns true if there is a regular file at the given path
### fs.remove

the fs.remove function removes a file or a directory recursively
### fs.glob

the fs.glob function takes a globbing path pattern argument (%./a/... will not work) and returns a list of paths matching this pattern.
### fs.find

the fs.find function takes a directory path argument followed by one or more globbing path patterns,  it returns a directory entry for each file matching at least one of the pattern.

**examples**

```inox
fs.find ./ %./**/*.json
```
### fs.get_tree_data

the fs.get_tree_data function takes a directory path argument and returns a %udata value  thats contains the file hiearachy of the passed directory.

**examples**

```inox
fs.get_tree_data(./)
```

## Functional

### map

the map function creates a list by applying an operation on each element of an iterable

**examples**

```inox
map([{name: "foo"}], .name)
# output: 
["foo"]
```
```inox
map([{a: 1, b: 2, c: 3}], .{a,b})
# output: 
[{a: 1, b: 2}]
```
```inox
map([0, 1, 2], Mapping{0 => "0" 1 => "1"})
# output: 
["0", "1", nil]
```
```inox
map([97, 98, 99], torune)
# output: 
['a', 'b', 'c']
```
```inox
map([0, 1, 2], @($ + 1))
# output: 
[1, 2, 3]
```
### filter

the filter function creates a list by iterating over an iterable and keeping elements that pass a condition

**examples**

```inox
filter(["a", "0", 1], %int)
# output: 
[1]
```
```inox
filter([0, 1, 2], @($ >= 1))
# output: 
[1, 2]
```
### some

the some function returns true if and only if at least one element of an iterable passes a condition. For an empty iterable the result is always true.

**examples**

```inox
some(["a", "0", 1], %int)
# output: 
true
```
```inox
some([0, 1, 2], @($ == 'a'))
# output: 
false
```
### all

the all function returns true if and only if all elements of an iterable pass a condition. For an empty iterable the result is always true.

**examples**

```inox
all([0, 1, "a"], %int)
# output: 
false
```
```inox
all([0, 1, 2], @($ >= 0))
# output: 
true
```
### none

the none function returns true if and only if no elements of an iterable pass a condition. For an emptty iterable the result is always true.

**examples**

```inox
none([0, 1, "a"], %int)
# output: 
false
```
```inox
none([0, 1, 2], @($ < 0))
# output: 
true
```
### sort

the sort function creates a new list by sorting a list of strings or integers, the second argument is an identifier describing the order. For strings the available orderings are #lex (lexicographic) & #revlex (same but reversed). For integers the available orderings are #asc (ascending) & #desc (descending).

**examples**

```inox
sort([2, 1], #asc)
# output: 
[1, 2]
```
```inox
sort(["b", "a"], #lex)
# output: 
["a", "b"]
```
### find

the find function searches for items matching a pattern at a given location (a string, an iterable, a directory)

**examples**

```inox
find %`a+` "a-aa-aaa"
# output: 
["a", "aa", "aaa"]
```
```inox
find %./**/*.json ./
# output: 
[./file.json, ./dir/file.json, ./dir/dir/.file.json]
```
```inox
find %int ['1', 2, "3"]
# output: 
[2]
```
### idt

the idt (identity) function takes a single argument and returns it.

## Html

### html.h1

the html.h1 function creates a h1 HTML element
### html.h2

the html.h2 function creates a h2 HTML element
### html.h3

the html.h3 function creates a h3 HTML element
### html.h4

the html.h4 function creates a h4 HTML element

## Http

### http

the http namespace contains functions to read, modify & delete HTTP resources. Most functions accept the --insecure option to ignore certificate errors & the --client option to specify a HTTP client to use.
### http.get

the http.get function takes a URL (or host) as first argument and returns a HTTP response. The --insecure options causes the function to ignore certificate errors.

**examples**

```inox
http.get https://example.com/
```
### http.read

the http.read function behaves exactly like the read function but only works on HTTP resources.

**examples**

```inox
https://jsonplaceholder.typicode.com/posts/1
```
### http.exists

the http.exists takes a URL (or host) as argument, it sends a HEAD request and returns true if the status code is less than 400.
### http.post

the http.post sends a POST request to the specified URL (or host) with the given body value, the body value can be any %readable or serializable object/list. A %mimetype value can be specified to change the value of the Content-Type header.

**examples**

```inox
http.post https://example.com/posts '{"title":"hello"}'
```
```inox
http.post https://example.com/posts {title: "hello"}
```
```inox
http.post https://example.com/posts [ {title: "hello"} ]
```
```inox
http.post https://example.com/posts mime"json" '{"title":"hello"}'
```
### http.patch

the http.patch function works like http.post but sends a HTTP PATCH request instead
### http.delete

the http.delete function sends a HTTP DELETE request to the specified URL
### http.Client

the http.Client function creates a HTTP client that can be used in most http.* functions with the --client flag.

**examples**

```inox
http.Client{ save-cookies: true }
```
```inox
http.Client{ insecure: true }
```
```inox
http.Client{
  request-finalization: :{
    https://example.com : { 
      add-headers: {X-API-KEY: env.initial.API_KEY}
    }
  } 
}

```
### http.Server

the http.Server function creates a listening HTTP server with a given host & handler. The handler can be an function or a Mapping that routes requests. When you send a request to a server listening to https://localhost add the --insecure flag to ignore certificate errors.

**examples**

```inox
fn handle(rw %http.resp_writer, r %http.req){
  rw.write_json({ a: 1 })
}

server = http.Server!(https://localhost:8080, Mapping {
    /hello => "hello"
    %/... => handle
})

```
```inox
fn handle(rw %http.resp_writer, r %http.req){
    match r.path {
      / {
          rw.write_json({ a: 1 })
      }
      %/... {
        rw.write_status(404)
      }
    }
}

server = http.Server!(https://localhost:8080, handle)

```
### http.FileServer

the http.FileServer creates a HTTP server that serves static file from a given directory.

**examples**

```inox
http.FileServer!(https://localhost:8080, ./examples/static/)
```
### http.servefile



## Rand

### rand

the rand function generates/pick a random value in a cryptographically secure way. If the argument is a pattern a matching value is returned, if the argument is an indexable an element is picked.

**examples**

```inox
rand(%int(0..10))
# output: 
3
```
```inox
rand(%str("a"+))
# output: 
"aaaaa"
```
```inox
rand(["a", "b"])
# output: 
"b"
```

## Tcp

### tcp.connect

the tcp.connect function creates a TCP connection to a given host.

**examples**

```inox
conn = tcp.connect!(://example.com:80)

conn.write!("GET / HTTP/1.1\nHost: example.com\n\n")
print tostr(conn.read!())

conn.close()


```

