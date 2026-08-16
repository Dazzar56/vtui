#pragma once

#include <string>
#include <vector>
#include <functional>
#include <map>
#include <set>
#include <sstream>
#include <iostream>

#include "../../c/include/vtui.h"

namespace vtui {

struct DialogOptions {
    int w = 40;
    int h = 10;
};

class Ui {
public:
    explicit Ui(vtui_session* session) : session_(session) {}

    class DialogScope {
    public:
        DialogScope(Ui& ui, const std::string& title, const DialogOptions& opt) : ui_(ui) {
            ui_.beginDialog(title, opt.w, opt.h);
        }
        ~DialogScope() {
            ui_.endDialog();
        }
    private:
        Ui& ui_;
    };

    DialogScope dialog(const std::string& title, const DialogOptions& opt = {}) {
        return DialogScope(*this, title, opt);
    }

    std::string edit(const std::string& label, const std::string& defVal = "") {
        std::string id = "edit_" + sanitizeId(label);
        if (values_.find(id) == values_.end()) {
            values_[id] = defVal;
        }
        addFormEdit(label, id, values_[id]);
        return values_[id];
    }

    bool button(const std::string& text) {
        std::string id = "btn_" + sanitizeId(text);
        addButton(text, id);
        return clickedIds_.find(id) != clickedIds_.end();
    }

    bool checkbox(const std::string& text, bool defVal = false) {
        std::string id = "chk_" + sanitizeId(text);
        if (values_.find(id) == values_.end()) {
            values_[id] = defVal ? "1" : "0";
        }
        addCheckbox(text, id, values_[id] == "1");
        return values_[id] == "1";
    }

    void message(const std::string& title, const std::string& text) {
        std::ostringstream ss;
        ss << "{\"op\":\"message\",\"title\":\"" << title << "\",\"text\":\"" << text << "\",\"buttons\":[\"&Ok\"]}\n";
        std::string msg = ss.str();
        vtui_send(session_, msg.c_str(), msg.size());
    }

    void beginDialog(const std::string& title, int w, int h) {
        currentChildren_.clear();
        dialogTitle_ = title;
        dialogW_ = w;
        dialogH_ = h;
    }

    void endDialog() {
        std::ostringstream ss;
        ss << "{\"op\":\"mount\",\"frameId\":\"mainDlg\",\"tree\":{"
           << "\"type\":\"Dialog\",\"id\":\"mainDlg\",\"props\":{\"title\":\"" << dialogTitle_ << "\",\"autoSize\":true,\"center\":true},"
           << "\"layout\":{\"type\":\"VBox\",\"spacing\":1,\"margins\":[1,2,1,2]},"
           << "\"children\":[" << currentChildren_ << "]}}\n";
        std::string line = ss.str();
        if (!mounted_) {
            vtui_send(session_, line.c_str(), line.size());
            mounted_ = true;
        }
        clickedIds_.clear();
    }

    void processEvent(const std::string& op, const std::string& srcId, const std::string& val) {
        if (op == "command" && !srcId.empty()) {
            clickedIds_.insert(srcId);
        } else if (op == "changed" && !srcId.empty()) {
            values_[srcId] = val;
        }
    }

private:
    std::string sanitizeId(const std::string& s) {
        std::string out;
        for (char c : s) {
            if (c != '&' && c != ' ') out += c;
        }
        return out;
    }

    void addFormEdit(const std::string& label, const std::string& id, const std::string& val) {
        if (!currentChildren_.empty()) currentChildren_ += ",";
        currentChildren_ += "{\"type\":\"Group\",\"layout\":{\"type\":\"Form\",\"spacing\":1},\"children\":["
                            "{\"type\":\"Label\",\"props\":{\"text\":\"" + label + "\",\"buddy\":\"" + id + "\"}},"
                            "{\"type\":\"Edit\",\"id\":\"" + id + "\",\"props\":{\"text\":\"" + val + "\"}}]}";
    }

    void addButton(const std::string& text, const std::string& id) {
        if (!currentChildren_.empty()) currentChildren_ += ",";
        currentChildren_ += "{\"type\":\"Button\",\"id\":\"" + id + "\",\"props\":{\"text\":\"" + text + "\",\"command\":1000}}";
    }

    void addCheckbox(const std::string& text, const std::string& id, bool checked) {
        if (!currentChildren_.empty()) currentChildren_ += ",";
        currentChildren_ += "{\"type\":\"Checkbox\",\"id\":\"" + id + "\",\"props\":{\"text\":\"" + text + "\",\"state\":" + (checked ? "1" : "0") + "}}";
    }

    vtui_session* session_;
    std::string dialogTitle_;
    int dialogW_ = 40;
    int dialogH_ = 10;
    std::string currentChildren_;
    std::map<std::string, std::string> values_;
    std::set<std::string> clickedIds_;
    bool mounted_ = false;
};

inline int run(std::function<void(Ui&)> ui_fn) {
    vtui_session* s = vtui_open("{\"backend\":\"ansi\"}");
    if (!s) {
        std::cerr << "Failed to open vtui session: " << vtui_last_error() << std::endl;
        return 1;
    }

    Ui u(s);
    ui_fn(u);

    char buf[4096];
    size_t out_len = 0;
    while (vtui_recv(s, buf, sizeof(buf), &out_len) == 0) {
        if (out_len > 0) {
            ui_fn(u);
        }
    }

    vtui_close(s);
    return 0;
}

} // namespace vtui
