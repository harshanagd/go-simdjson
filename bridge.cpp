// bridge.cpp — C bridge implementation for simdjson.

#include "bridge.h"
#include "simdjson.h"

struct parser_state {
    simdjson::dom::parser parser;
    simdjson::dom::element root;
    bool has_doc;
};

extern "C" {

simdjson_parser simdjson_parser_new(void) {
    return new parser_state{};
}

void simdjson_parser_free(simdjson_parser p) {
    delete static_cast<parser_state*>(p);
}

simdjson_result simdjson_parse(simdjson_parser p, const char* buf, size_t len) {
    auto* state = static_cast<parser_state*>(p);
    state->has_doc = false;
    auto error = state->parser.parse(buf, len).get(state->root);
    if (error) {
        return {0, static_cast<int>(error), simdjson::error_message(error)};
    }
    state->has_doc = true;
    return {1, 0, nullptr};
}

int simdjson_root_type(simdjson_parser p) {
    auto* state = static_cast<parser_state*>(p);
    if (!state->has_doc) return -1;
    return static_cast<int>(state->root.type());
}

int simdjson_find_string(simdjson_parser p, const char* key, size_t key_len,
                         const char** out_str, size_t* out_len) {
    auto* state = static_cast<parser_state*>(p);
    if (!state->has_doc) return -1;

    simdjson::dom::object obj;
    auto error = state->root.get(obj);
    if (error) return static_cast<int>(error);

    std::string_view k(key, key_len);
    std::string_view val;
    error = obj[k].get(val);
    if (error) return static_cast<int>(error);

    *out_str = val.data();
    *out_len = val.size();
    return 0;
}

int simdjson_get_root_string(simdjson_parser p, const char** out_str, size_t* out_len) {
    auto* state = static_cast<parser_state*>(p);
    if (!state->has_doc) return -1;
    std::string_view val;
    auto error = state->root.get(val);
    if (error) return static_cast<int>(error);
    *out_str = val.data();
    *out_len = val.size();
    return 0;
}

int simdjson_get_root_int64(simdjson_parser p, int64_t* out) {
    auto* state = static_cast<parser_state*>(p);
    if (!state->has_doc) return -1;
    auto error = state->root.get(*out);
    if (error) return static_cast<int>(error);
    return 0;
}

int simdjson_get_root_uint64(simdjson_parser p, uint64_t* out) {
    auto* state = static_cast<parser_state*>(p);
    if (!state->has_doc) return -1;
    auto error = state->root.get(*out);
    if (error) return static_cast<int>(error);
    return 0;
}

int simdjson_get_root_double(simdjson_parser p, double* out) {
    auto* state = static_cast<parser_state*>(p);
    if (!state->has_doc) return -1;
    auto error = state->root.get(*out);
    if (error) return static_cast<int>(error);
    return 0;
}

int simdjson_get_root_bool(simdjson_parser p, int* out) {
    auto* state = static_cast<parser_state*>(p);
    if (!state->has_doc) return -1;
    bool val;
    auto error = state->root.get(val);
    if (error) return static_cast<int>(error);
    *out = val ? 1 : 0;
    return 0;
}

int simdjson_root_count(simdjson_parser p, size_t* out) {
    auto* state = static_cast<parser_state*>(p);
    if (!state->has_doc) return -1;

    if (state->root.type() == simdjson::dom::element_type::ARRAY) {
        simdjson::dom::array arr;
        auto error = state->root.get(arr);
        if (error) return static_cast<int>(error);
        *out = arr.size();
        return 0;
    }
    if (state->root.type() == simdjson::dom::element_type::OBJECT) {
        simdjson::dom::object obj;
        auto error = state->root.get(obj);
        if (error) return static_cast<int>(error);
        *out = obj.size();
        return 0;
    }
    return -1;
}

} // extern "C"

extern "C" const char* simdjson_active_implementation(void) {
    return simdjson::get_active_implementation()->name().data();
}
