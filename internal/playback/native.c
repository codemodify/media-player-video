//go:build cgo

#include "native.h"
#include <stdio.h>
#include <string.h>
#ifdef _WIN32
#include <windows.h>
#define VP_LOAD(name) LoadLibraryA(name)
#define VP_SYMBOL(handle, name) GetProcAddress(handle, name)
#define VP_UNLOAD(handle) FreeLibrary(handle)
#else
#include <dlfcn.h>
#define VP_LOAD(name) dlopen(name, RTLD_NOW | RTLD_LOCAL)
#define VP_SYMBOL(handle, name) dlsym(handle, name)
#define VP_UNLOAD(handle) dlclose(handle)
#endif

struct vp_api {
	void *library;
	mpv_handle *(*create)(void);
	int (*set_option_string)(mpv_handle *, const char *, const char *);
	int (*initialize)(mpv_handle *);
	void (*terminate_destroy)(mpv_handle *);
	int (*command)(mpv_handle *, const char **);
	mpv_event *(*wait_event)(mpv_handle *, double);
	int (*get_property)(mpv_handle *, const char *, mpv_format, void *);
	char *(*get_property_string)(mpv_handle *, const char *);
	void (*free)(void *);
	void (*free_node_contents)(mpv_node *);
	const char *(*error_string)(int);
	int (*render_context_create)(mpv_render_context **, mpv_handle *, mpv_render_param *);
	void (*render_context_free)(mpv_render_context *);
	uint64_t (*render_context_update)(mpv_render_context *);
	int (*render_context_render)(mpv_render_context *, mpv_render_param *);
};

vp_api *vp_open(const char *path, char *error, int length) {
	vp_api *a = calloc(1, sizeof(*a));
	if (!a) { snprintf(error, length, "out of memory"); return NULL; }
	a->library = VP_LOAD(path);
	if (!a->library) { snprintf(error, length, "could not load %s", path); free(a); return NULL; }
#define LOAD(field) do { *(void **)(&a->field) = (void *)VP_SYMBOL(a->library, "mpv_" #field); if (!a->field) { snprintf(error, length, "%s is missing mpv_%s", path, #field); vp_close(a); return NULL; } } while (0)
	LOAD(create); LOAD(set_option_string); LOAD(initialize); LOAD(terminate_destroy);
	LOAD(command); LOAD(wait_event); LOAD(get_property); LOAD(get_property_string);
	LOAD(free); LOAD(free_node_contents); LOAD(error_string); LOAD(render_context_create);
	LOAD(render_context_free); LOAD(render_context_update); LOAD(render_context_render);
#undef LOAD
	return a;
}
void vp_close(vp_api *a) { if (a) { VP_UNLOAD(a->library); free(a); } }
mpv_handle *vp_create(vp_api *a) { return a->create(); }
int vp_option(vp_api *a, mpv_handle *h, const char *n, const char *v) { return a->set_option_string(h,n,v); }
int vp_initialize(vp_api *a, mpv_handle *h) { return a->initialize(h); }
void vp_destroy(vp_api *a, mpv_handle *h) { a->terminate_destroy(h); }
int vp_command(vp_api *a, mpv_handle *h, const char **args) { return a->command(h,args); }
mpv_event *vp_event(vp_api *a, mpv_handle *h) { return a->wait_event(h,0); }
double vp_number(vp_api *a, mpv_handle *h, const char *n, double fallback) { double v; return a->get_property(h,n,MPV_FORMAT_DOUBLE,&v) < 0 ? fallback : v; }
int64_t vp_integer(vp_api *a, mpv_handle *h, const char *n, int64_t fallback) { int64_t v; return a->get_property(h,n,MPV_FORMAT_INT64,&v) < 0 ? fallback : v; }
int vp_flag(vp_api *a, mpv_handle *h, const char *n, int fallback) { int v; return a->get_property(h,n,MPV_FORMAT_FLAG,&v) < 0 ? fallback : v; }
char *vp_string(vp_api *a, mpv_handle *h, const char *n) { return a->get_property_string(h,n); }
void vp_free(vp_api *a, void *p) { a->free(p); }
const char *vp_error(vp_api *a, int code) { return a->error_string(code); }
int vp_render_create(vp_api *a, mpv_handle *h, mpv_render_context **context) {
	mpv_render_param p[] = {{MPV_RENDER_PARAM_API_TYPE, MPV_RENDER_API_TYPE_SW}, {0, NULL}};
	return a->render_context_create(context,h,p);
}
void vp_render_free(vp_api *a, mpv_render_context *r) { a->render_context_free(r); }
int vp_render_update(vp_api *a, mpv_render_context *r) { return (a->render_context_update(r) & MPV_RENDER_UPDATE_FRAME) != 0; }
int vp_render(vp_api *a, mpv_render_context *r, int w, int h, size_t stride, void *pixels) {
	int size[] = {w,h};
	mpv_render_param p[] = {{MPV_RENDER_PARAM_SW_SIZE, size}, {MPV_RENDER_PARAM_SW_FORMAT, "rgb0"}, {MPV_RENDER_PARAM_SW_STRIDE, &stride}, {MPV_RENDER_PARAM_SW_POINTER, pixels}, {0,NULL}};
	return a->render_context_render(r,p);
}
mpv_node *vp_tracks(vp_api *a, mpv_handle *h) {
	mpv_node *n = calloc(1,sizeof(*n));
	if (!n) return NULL;
	if (a->get_property(h,"track-list",MPV_FORMAT_NODE,n) < 0) { free(n); return NULL; }
	return n;
}
int vp_track_count(mpv_node *n) { return n && n->format == MPV_FORMAT_NODE_ARRAY ? n->u.list->num : 0; }
static mpv_node *field(mpv_node *n, int index, const char *key) {
	if (index < 0 || index >= vp_track_count(n)) return NULL;
	mpv_node *row = &n->u.list->values[index];
	if (row->format != MPV_FORMAT_NODE_MAP) return NULL;
	for (int i=0; i < row->u.list->num; i++) if (!strcmp(row->u.list->keys[i],key)) return &row->u.list->values[i];
	return NULL;
}
int vp_track_id(mpv_node *n, int i) { mpv_node *f=field(n,i,"id"); return f && f->format == MPV_FORMAT_INT64 ? (int)f->u.int64 : 0; }
int vp_track_selected(mpv_node *n, int i) { mpv_node *f=field(n,i,"selected"); return f && f->format == MPV_FORMAT_FLAG ? f->u.flag : 0; }
const char *vp_track_text(mpv_node *n, int i, const char *key) { mpv_node *f=field(n,i,key); return f && f->format == MPV_FORMAT_STRING ? f->u.string : ""; }
void vp_tracks_free(vp_api *a, mpv_node *n) { if (n) { a->free_node_contents(n); free(n); } }
