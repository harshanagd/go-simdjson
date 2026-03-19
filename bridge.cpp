// bridge.cpp — C bridge implementation for simdjson.

#include "bridge.h"
#include "simdjson.h"

#include <cstring>

struct parser_state {
    simdjson::dom::parser parser;
    simdjson::dom::element root;
    bool has_doc;
};

extern "C" {

// --- Parser lifecycle ---

simdjson_parser simdjson_parser_new(void) {
    return new parser_state{};
}

void simdjson_parser_free(simdjson_parser p) {
    delete static_cast<parser_state*>(p);
}

simdjson_parse_result simdjson_parse_and_get_tape(simdjson_parser p, const char* buf, size_t len) {
    simdjson_parse_result r = {};
    auto* state = static_cast<parser_state*>(p);
    state->has_doc = false;
    auto error = state->parser.parse(buf, len).get(state->root);
    if (error) {
        r.result = {0, static_cast<int>(error), simdjson::error_message(error)};
        return r;
    }
    state->has_doc = true;
    r.result = {1, 0, nullptr};

    auto& doc = state->parser.doc;
    if (!doc.tape) return r;

    uint64_t first = doc.tape[0];
    size_t tape_len_val = (first & 0x00ffffffffffffff);
    // tape_len_val points to the closing root entry; include it
    if (tape_len_val > 0) tape_len_val++;
    r.tape = doc.tape.get();
    r.tape_len = tape_len_val;
    r.sbuf = doc.string_buf.get();
    if (!r.sbuf) {
        r.sbuf_len = 0;
        return r;
    }
    size_t max_end = 0;
    // Scan tape for string entries to find actual string buffer usage.
    // Skip final root entry (i + 1 < tape_len_val).
    // Use 2x capacity as safety bound for string buffer reads.
    size_t sbuf_cap = doc.capacity() * 2 + 64;
    for (size_t i = 0; i + 1 < tape_len_val; i++) {
        uint8_t tag = doc.tape[i] >> 56;
        if (tag == '"') {
            uint64_t offset = doc.tape[i] & 0x00ffffffffffffff;
            if (offset + 4 > sbuf_cap) continue; // safety check
            uint32_t slen;
            memcpy(&slen, doc.string_buf.get() + offset, sizeof(uint32_t));
            size_t end = offset + 4 + slen + 1;
            if (end > sbuf_cap) continue; // safety check
            if (end > max_end) max_end = end;
        }
    }
    r.sbuf_len = max_end;
    return r;
}

// --- Runtime info ---

const char* simdjson_active_implementation(void) {
    static std::string name = simdjson::get_active_implementation()->name();
    return name.c_str();
}

} // extern "C"
