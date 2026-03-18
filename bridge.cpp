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

// --- Tape access ---

int simdjson_get_tape(simdjson_parser p,
                      const uint64_t** tape, size_t* tape_len,
                      const uint8_t** sbuf, size_t* sbuf_len) {
    auto* state = static_cast<parser_state*>(p);
    if (!state->has_doc) return -1;
    auto& doc = state->parser.doc;
    // Tape length: first root entry payload points past the last entry
    uint64_t first = doc.tape[0];
    size_t len = (first & 0x00ffffffffffffff) + 1;
    *tape = doc.tape.get();
    *tape_len = len;
    *sbuf = doc.string_buf.get();
    // Compute actual string buffer usage by finding max string end in tape
    size_t max_end = 0;
    for (size_t i = 0; i < len; i++) {
        uint8_t tag = doc.tape[i] >> 56;
        if (tag == '"') {
            uint64_t offset = doc.tape[i] & 0x00ffffffffffffff;
            uint32_t slen;
            memcpy(&slen, doc.string_buf.get() + offset, sizeof(uint32_t));
            size_t end = offset + 4 + slen + 1;
            if (end > max_end) max_end = end;
        }
    }
    *sbuf_len = max_end;
    return 0;
}

// --- Runtime info ---

const char* simdjson_active_implementation(void) {
    return simdjson::get_active_implementation()->name().data();
}

} // extern "C"
