#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <stdarg.h>

#include "../include/vtui.h"

struct vtui_ui {
    vtui_session *session;
    char dialog_title[128];
    int dialog_width;
    char current_children[4096];
    char last_cmd_src[64];
    int mounted;
};

void vtui_dialog(vtui_ui *u, const char *title, int w) {
    if (!u) return;
    strncpy(u->dialog_title, title ? title : "", sizeof(u->dialog_title) - 1);
    u->dialog_width = w > 0 ? w : 40;
    u->current_children[0] = '\0';
}

void vtui_edit(vtui_ui *u, const char *label, char *buf, size_t buf_cap) {
    if (!u) return;
    char entry[512];
    snprintf(entry, sizeof(entry),
        "%s{\"type\":\"Group\",\"layout\":{\"type\":\"Form\",\"spacing\":1},\"children\":["
        "{\"type\":\"Label\",\"props\":{\"text\":\"%s\"}},"
        "{\"type\":\"Edit\",\"id\":\"nameEdit\",\"props\":{\"text\":\"%s\"}}]}",
        (u->current_children[0] != '\0') ? "," : "",
        label ? label : "",
        buf ? buf : "");
    strncat(u->current_children, entry, sizeof(u->current_children) - strlen(u->current_children) - 1);
}

int vtui_button(vtui_ui *u, const char *text) {
    if (!u) return 0;
    char entry[256];
    snprintf(entry, sizeof(entry),
        "%s{\"type\":\"Button\",\"id\":\"okBtn\",\"props\":{\"text\":\"%s\",\"command\":1000}}",
        (u->current_children[0] != '\0') ? "," : "",
        text ? text : "&Ok");
    strncat(u->current_children, entry, sizeof(u->current_children) - strlen(u->current_children) - 1);
    return (strcmp(u->last_cmd_src, "okBtn") == 0);
}

int vtui_checkbox(vtui_ui *u, const char *text, int default_state) {
    if (!u) return 0;
    char entry[256];
    snprintf(entry, sizeof(entry),
        "%s{\"type\":\"Checkbox\",\"props\":{\"text\":\"%s\",\"state\":%d}}",
        (u->current_children[0] != '\0') ? "," : "",
        text ? text : "",
        default_state ? 1 : 0);
    strncat(u->current_children, entry, sizeof(u->current_children) - strlen(u->current_children) - 1);
    return default_state;
}

void vtui_message(vtui_ui *u, const char *title, const char *text) {
    if (!u || !u->session) return;
    char msg[512];
    snprintf(msg, sizeof(msg),
        "{\"op\":\"message\",\"title\":\"%s\",\"text\":\"%s\",\"buttons\":[\"&Ok\"]}\n",
        title ? title : " Message ", text ? text : "");
    vtui_send(u->session, msg, strlen(msg));
}

void vtui_end(vtui_ui *u) {
    if (!u || !u->session) return;
    if (!u->mounted) {
        char mount[8192];
        snprintf(mount, sizeof(mount),
            "{\"op\":\"mount\",\"frameId\":\"mainDlg\",\"tree\":{"
            "\"type\":\"Dialog\",\"id\":\"mainDlg\",\"props\":{\"title\":\"%s\",\"autoSize\":true,\"center\":true},"
            "\"layout\":{\"type\":\"VBox\",\"spacing\":1,\"margins\":[1,2,1,2]},"
            "\"children\":[%s]}}\n",
            u->dialog_title, u->current_children);
        vtui_send(u->session, mount, strlen(mount));
        u->mounted = 1;
    }
    u->last_cmd_src[0] = '\0';
}

int vtui_run(vtui_ui_func ui_fn) {
    if (!ui_fn) return 1;

    vtui_session *s = vtui_open("{\"backend\":\"ansi\"}");
    if (!s) {
        fprintf(stderr, "vtui_open failed: %s\n", vtui_last_error());
        return 1;
    }

    vtui_ui u;
    memset(&u, 0, sizeof(u));
    u.session = s;

    ui_fn(&u);

    char buf[4096];
    size_t out_len = 0;
    while (vtui_recv(s, buf, sizeof(buf) - 1, &out_len) == 0) {
        if (out_len > 0) {
            buf[out_len] = '\0';
            if (strstr(buf, "\"op\":\"closed\"") != NULL) {
                break;
            }
            if (strstr(buf, "\"op\":\"command\"") != NULL) {
                char *src = strstr(buf, "\"srcId\":\"");
                if (src) {
                    src += 9;
                    char *end = strchr(src, '"');
                    if (end) {
                        size_t len = end - src;
                        if (len >= sizeof(u.last_cmd_src)) len = sizeof(u.last_cmd_src) - 1;
                        strncpy(u.last_cmd_src, src, len);
                        u.last_cmd_src[len] = '\0';
                    }
                }
            }
            ui_fn(&u);
        }
    }

    vtui_close(s);
    return 0;
}
