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

simdjson_parse_result simdjson_parse_and_get_tape(simdjson_parser p, const char* buf, size_t len, int number_as_string) {
    simdjson_parse_result r = {};
    auto* state = static_cast<parser_state*>(p);
    state->has_doc = false;
    state->parser.number_as_string(number_as_string != 0);
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
        if (tag == '"' || tag == 'Z') {
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

// --- On Demand tape ---
// (reserved for future use)

// --- NDJSON via parse_many ---

simdjson_nd_result simdjson_parse_many(simdjson_parser p, const char* buf, size_t len) {
    simdjson_nd_result r = {};
    auto* state = static_cast<parser_state*>(p);

    auto padded = simdjson::padded_string(buf, len);
    simdjson::dom::document_stream docs;
    auto error = state->parser.parse_many(padded).get(docs);
    if (error) {
        r.result.ok = 0;
        r.result.error_code = static_cast<int>(error);
        r.result.error_msg = simdjson::error_message(error);
        return r;
    }

    // Collect all document tapes into one combined tape + string buffer.
    thread_local std::vector<uint64_t> combined_tape;
    thread_local std::vector<uint8_t> combined_strings;
    combined_tape.clear();
    combined_strings.clear();

    for (auto doc_result : docs) {
        simdjson::dom::element doc;
        error = doc_result.get(doc);
        if (error) {
            r.result.ok = 0;
            r.result.error_code = static_cast<int>(error);
            r.result.error_msg = simdjson::error_message(error);
            return r;
        }

        auto& d = state->parser.doc;
        if (!d.tape) continue;

        uint64_t first = d.tape[0];
        size_t tape_len = (first & 0x00ffffffffffffff);
        if (tape_len > 0) tape_len++;

        // Adjust tape offsets: string offsets need to shift by current combined_strings size.
        // Container end-indices need to shift by current combined_tape size.
        size_t tape_base = combined_tape.size();
        size_t str_base = combined_strings.size();

        for (size_t i = 0; i < tape_len; i++) {
            uint64_t entry = d.tape[i];
            uint8_t tag = entry >> 56;
            uint64_t payload = entry & 0x00ffffffffffffff;

            switch (tag) {
            case '"': // string: payload is string buffer offset
                entry = (uint64_t(tag) << 56) | (payload + str_base);
                break;
            case '{': case '[': // container open: lower 32 bits = end index
            {
                uint32_t end_idx = uint32_t(payload) + uint32_t(tape_base);
                uint32_t count = uint32_t(payload >> 32);
                entry = (uint64_t(tag) << 56) | (uint64_t(count) << 32) | end_idx;
                break;
            }
            case '}': case ']': // container close: payload = start index
                entry = (uint64_t(tag) << 56) | (payload + tape_base);
                break;
            case 'r': // root: payload = other root index
                entry = (uint64_t(tag) << 56) | (payload + tape_base);
                break;
            default:
                break;
            }
            combined_tape.push_back(entry);
        }

        // Copy string buffer
        if (d.string_buf) {
            // Find actual string buffer usage (same scan as parse_and_get_tape)
            size_t max_end = 0;
            size_t sbuf_cap = d.capacity() * 2 + 64;
            for (size_t i = 0; i + 1 < tape_len; i++) {
                uint8_t t = d.tape[i] >> 56;
                if (t == '"') {
                    uint64_t off = d.tape[i] & 0x00ffffffffffffff;
                    if (off + 4 > sbuf_cap) continue;
                    uint32_t slen;
                    memcpy(&slen, d.string_buf.get() + off, sizeof(uint32_t));
                    size_t end = off + 4 + slen + 1;
                    if (end > sbuf_cap) continue;
                    if (end > max_end) max_end = end;
                }
            }
            size_t old_size = combined_strings.size();
            combined_strings.resize(old_size + max_end);
            memcpy(combined_strings.data() + old_size, d.string_buf.get(), max_end);
        }
    }

    r.result = {1, 0, nullptr};
    r.tape = combined_tape.data();
    r.tape_len = combined_tape.size();
    r.sbuf = combined_strings.data();
    r.sbuf_len = combined_strings.size();
    return r;
}

// --- Runtime info ---

const char* simdjson_active_implementation(void) {
    static std::string name = simdjson::get_active_implementation()->name();
    return name.c_str();
}

} // extern "C"
