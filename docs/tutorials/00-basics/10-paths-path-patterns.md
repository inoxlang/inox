# Paths and Path patterns 

```
manifest {}

# ====== paths ======

absolute_filepath = /file.txt
relative_filepath1 = ./file.txt
relative_filepath2 = ../file.txt

# Directory paths end with a slash.
absolute_dirpath = /dir/

path = absolute_dirpath.join(./file.txt)
print("(/dir/).join(./file.txt):", path)

# If a path contains spaces or delimiters such as `[` or `]` it should be quoted:
quoted_path = /`[file].txt`

# ====== path patterns ======

# Prefix path patterns end with /...
prefix_path_pattern = %/...

print "\n(/ match %/...):" (/ match %/...)
print "(/file.txt match %/...):" (/file.txt match %/...)
print "(/dir/file.txt match %/...):" (/dir/file.txt match %/...)
print "(/file.txt match %/dir/...):" (/file.txt match %/dir/...)

# Path patterns that do not end with /... are glob path patterns.
glob_path_pattern = %/*.json

print "\n(/ match %/*.json):" (/ match %/*.json)
print "(/file.json match %/*.json):" (/file.json match %/*.json)
print "(/dir/file.json match %/*.json):" (/dir/file.json match %/*.json)

# You can learn more about path patterns in the language reference: 
# https://github.com/inoxlang/inox/blob/main/docs/language-reference/language.md#path-patterns.
```