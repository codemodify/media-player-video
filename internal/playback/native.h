#ifndef VIDEO_PLAYER_NATIVE_H
#define VIDEO_PLAYER_NATIVE_H
#include "mpvinclude/client.h"
#include "mpvinclude/render.h"
#include <stdlib.h>

typedef struct vp_api vp_api;
vp_api *vp_open(const char *path, char *error, int length);
void vp_close(vp_api *api);
mpv_handle *vp_create(vp_api *api);
int vp_option(vp_api *api, mpv_handle *mpv, const char *name, const char *value);
int vp_initialize(vp_api *api, mpv_handle *mpv);
void vp_destroy(vp_api *api, mpv_handle *mpv);
int vp_command(vp_api *api, mpv_handle *mpv, const char **args);
mpv_event *vp_event(vp_api *api, mpv_handle *mpv);
double vp_number(vp_api *api, mpv_handle *mpv, const char *name, double fallback);
int64_t vp_integer(vp_api *api, mpv_handle *mpv, const char *name, int64_t fallback);
int vp_flag(vp_api *api, mpv_handle *mpv, const char *name, int fallback);
char *vp_string(vp_api *api, mpv_handle *mpv, const char *name);
void vp_free(vp_api *api, void *ptr);
const char *vp_error(vp_api *api, int code);
int vp_render_create(vp_api *api, mpv_handle *mpv, mpv_render_context **context);
void vp_render_free(vp_api *api, mpv_render_context *context);
int vp_render_update(vp_api *api, mpv_render_context *context);
int vp_render(vp_api *api, mpv_render_context *context, int w, int h, size_t stride, void *pixels);
mpv_node *vp_tracks(vp_api *api, mpv_handle *mpv);
int vp_track_count(mpv_node *node);
int vp_track_id(mpv_node *node, int index);
int vp_track_selected(mpv_node *node, int index);
const char *vp_track_text(mpv_node *node, int index, const char *field);
void vp_tracks_free(vp_api *api, mpv_node *node);
#endif
