# terraform-source-hash

Calculate a hash from terraform files to detect changes.

## How to use

### Install

```
go install github.com/ka2n/terraform-source-hash@latest
```

### Use

```
$ cd <terraform root module dir>
$ terraform-source-hash .
0009e2dbd4d92fb16c7cc7438532a2963bede55

# Make any changes(include comments!)
$ echo "# Hello" > foo.tf
$ terraform-source-hash .
b6bd87af779ba9d193f724782afda6e5155b5c9
```

## Changes include

- `.tf` files
- Local module files
- Remote module name and version
