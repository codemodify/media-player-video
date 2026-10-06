# libmpv API headers

`client.h` and `render.h` are unmodified libmpv public headers from the installed
development package, client API version 2.5. Their ISC copyright and permission
notices are retained at the beginning of each file. The corresponding upstream
headers are [client.h](https://github.com/mpv-player/mpv/blob/master/libmpv/client.h)
and [render.h](https://github.com/mpv-player/mpv/blob/master/libmpv/render.h).

The player compiles against these declarations and loads the shared library's
functions at runtime. The library is not bundled. It must provide the client
and software render APIs, including playlist entry IDs and software RGB output.
The runtime mpv core has its own distribution license, as described in the
headers and its upstream distribution.
