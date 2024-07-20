# lscolors

**NOTE:** This is still a work in progress and the API should be considered
unstable.

lscolors is a package for parsing the [LS_COLORS](https://man7.org/linux/man-pages/man1/dircolors.1.html)
environment variable and matching it against file types. This is for Go programs
that want to print colorized file / directory info in the same color scheme as
[GNU ls](https://man7.org/linux/man-pages/man1/ls.1.html).
