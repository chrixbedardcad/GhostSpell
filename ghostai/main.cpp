// ghostai — GhostSpell LLM inference helper.
// Pure C++ binary that links llama.cpp directly (no CGo).
//
// Daemon mode (persistent process, model stays loaded):
//   ghostai --daemon -m <model> [-t <threads>] [--gpu-layers <n>] [--context-size <n>]
//   Then send JSON commands on stdin, one per line:
//     {"messages":[{"role":"system","content":"..."},{"role":"user","content":"..."}],"max_tokens":256}
//   Receive JSON responses on stdout, one per line:
//     {"text":"...","model":"...","prompt_tokens":10,"completion_tokens":50,"tokens_per_second":45.2}
//   Send {"quit":true} or close stdin to exit.

#include "llama.h"

#include <cstdio>
#include <cstdlib>
#include <cstring>
#include <cctype>
#include <string>
#include <vector>
#include <algorithm>

#ifdef _WIN32
#include <windows.h>
static double now_ms() {
    static LARGE_INTEGER freq = {};
    if (freq.QuadPart == 0) QueryPerformanceFrequency(&freq);
    LARGE_INTEGER t;
    QueryPerformanceCounter(&t);
    return (double)t.QuadPart / freq.QuadPart * 1000.0;
}
#else
#include <time.h>
#include <signal.h>
#include <unistd.h>
static double now_ms() {
    struct timespec ts;
    clock_gettime(CLOCK_MONOTONIC, &ts);
    return ts.tv_sec * 1000.0 + ts.tv_nsec / 1e6;
}
#endif

// ---------------------------------------------------------------------------
// Simple JSON helpers (no library needed for our fixed format).
// ---------------------------------------------------------------------------

static std::string json_get_string(const std::string &json, const char *key) {
    std::string needle = std::string("\"") + key + "\":\"";
    auto pos = json.find(needle);
    if (pos == std::string::npos) return "";
    pos += needle.size();
    auto end = json.find("\"", pos);
    if (end == std::string::npos) return "";
    std::string result;
    for (size_t i = pos; i < end; i++) {
        if (json[i] == '\\' && i + 1 < end) {
            char next = json[i + 1];
            if (next == '\\') { result += '\\'; i++; }
            else if (next == '"') { result += '"'; i++; }
            else if (next == 'n') { result += '\n'; i++; }
            else if (next == 'r') { result += '\r'; i++; }
            else if (next == 't') { result += '\t'; i++; }
            else if (next == '/') { result += '/'; i++; }
            else result += json[i];
        } else {
            result += json[i];
        }
    }
    return result;
}

static int json_get_int(const std::string &json, const char *key, int def) {
    std::string needle = std::string("\"") + key + "\":";
    auto pos = json.find(needle);
    if (pos == std::string::npos) return def;
    pos += needle.size();
    while (pos < json.size() && json[pos] == ' ') pos++;
    return atoi(json.c_str() + pos);
}

static bool json_has_key(const std::string &json, const char *key) {
    return json.find(std::string("\"") + key + "\"") != std::string::npos;
}

static std::string json_escape(const std::string &s) {
    std::string out;
    for (char c : s) {
        if (c == '"') out += "\\\"";
        else if (c == '\\') out += "\\\\";
        else if (c == '\n') out += "\\n";
        else if (c == '\r') out += "\\r";
        else if (c == '\t') out += "\\t";
        else out += c;
    }
    return out;
}

// Extract all {"role":"...","content":"..."} pairs from a messages array.
struct chat_message {
    std::string role;
    std::string content;
};

static std::vector<chat_message> parse_messages(const std::string &json) {
    std::vector<chat_message> msgs;
    size_t pos = 0;
    while ((pos = json.find("\"role\"", pos)) != std::string::npos) {
        // Find the enclosing object boundaries.
        size_t obj_start = json.rfind('{', pos);
        size_t obj_end = json.find('}', pos);
        if (obj_start == std::string::npos || obj_end == std::string::npos) break;
        std::string obj = json.substr(obj_start, obj_end - obj_start + 1);
        chat_message msg;
        msg.role = json_get_string(obj, "role");
        msg.content = json_get_string(obj, "content");
        if (!msg.role.empty()) msgs.push_back(msg);
        pos = obj_end + 1;
    }
    return msgs;
}

// ---------------------------------------------------------------------------
// String utilities for output cleaning.
// ---------------------------------------------------------------------------

static std::string str_tolower(const std::string &s) {
    std::string out = s;
    for (auto &c : out) c = (char)tolower((unsigned char)c);
    return out;
}

static std::string str_trim(const std::string &s) {
    size_t start = s.find_first_not_of(" \t\n\r");
    if (start == std::string::npos) return "";
    size_t end = s.find_last_not_of(" \t\n\r");
    return s.substr(start, end - start + 1);
}

// Remove all <think>...</think> blocks from text.
static std::string strip_thinking_tags(const std::string &s) {
    std::string out = s;
    for (;;) {
        auto start = out.find("<think>");
        if (start == std::string::npos) break;
        auto end = out.find("</think>", start);
        if (end == std::string::npos) {
            // Unclosed <think> — remove everything from <think> onward.
            out = out.substr(0, start);
            break;
        }
        out = out.substr(0, start) + out.substr(end + 8);
    }
    return out;
}

// Remove special tokens and model turn markers.
static std::string clean_special_tokens(const std::string &s) {
    std::string out = s;
    const char *tokens[] = {
        "<|im_end|>", "<|im_start|>", "</s>", "<|endoftext|>",
        "<|end|>", "/no_think", nullptr
    };
    for (int i = 0; tokens[i]; i++) {
        size_t pos;
        while ((pos = out.find(tokens[i])) != std::string::npos) {
            out.erase(pos, strlen(tokens[i]));
        }
    }
    // Truncate at model turn markers.
    const char *markers[] = {"\nUser:", "\nAssistant:", "\nHuman:", "\n<|", nullptr};
    for (int i = 0; markers[i]; i++) {
        auto pos = out.find(markers[i]);
        if (pos != std::string::npos) out = out.substr(0, pos);
    }
    return out;
}

static std::string clean_response(const std::string &raw) {
    std::string s = strip_thinking_tags(raw);
    s = clean_special_tokens(s);
    return str_trim(s);
}

// ---------------------------------------------------------------------------
// Thinking model detection.
// ---------------------------------------------------------------------------

static bool is_thinking_model(const std::string &name) {
    std::string lower = str_tolower(name);
    return lower.find("qwen3") != std::string::npos ||
           lower.find("deepseek") != std::string::npos;
}

static bool is_qwen35(const std::string &name) {
    return str_tolower(name).find("qwen3.5") != std::string::npos;
}

// ---------------------------------------------------------------------------
// Abort flag — checked between tokens via llama callback.
// ---------------------------------------------------------------------------

static volatile int g_abort_flag = 0;

static bool abort_callback(void *data) {
    (void)data;
    return g_abort_flag != 0;
}

// ---------------------------------------------------------------------------
// Core inference.
// ---------------------------------------------------------------------------

struct completion_result {
    std::string text;
    int prompt_tokens;
    int completion_tokens;
    double tokens_per_second;
};

static completion_result complete(
    struct llama_model *model,
    const struct llama_vocab *vocab,
    const std::string &prompt,
    int max_tokens,
    int n_threads,
    int context_size,
    int batch_size
) {
    completion_result result = {};
    g_abort_flag = 0;

    // Create context for this inference.
    struct llama_context_params cparams = llama_context_default_params();
    cparams.n_ctx = context_size;
    cparams.n_batch = batch_size;
    cparams.n_threads = n_threads;
    cparams.n_threads_batch = n_threads;
    cparams.abort_callback = abort_callback;
    cparams.abort_callback_data = nullptr;

    struct llama_context *ctx = llama_init_from_model(model, cparams);
    if (!ctx) {
        result.text = "";
        return result;
    }

    // Tokenize.
    int n_prompt_max = (int)prompt.size() + 256;
    std::vector<llama_token> tokens(n_prompt_max);
    int n_prompt = llama_tokenize(vocab, prompt.c_str(), (int)prompt.size(),
                                   tokens.data(), n_prompt_max, true, true);
    if (n_prompt < 0) {
        tokens.resize(-n_prompt);
        n_prompt = llama_tokenize(vocab, prompt.c_str(), (int)prompt.size(),
                                   tokens.data(), -n_prompt, true, true);
    }
    if (n_prompt <= 0) { llama_free(ctx); return result; }
    tokens.resize(n_prompt);
    result.prompt_tokens = n_prompt;

    // Check context window.
    if (n_prompt + max_tokens > context_size) {
        max_tokens = context_size - n_prompt;
        if (max_tokens <= 0) { llama_free(ctx); return result; }
    }

    // Prefill prompt.
    double t_start = now_ms();
    struct llama_batch batch = llama_batch_get_one(tokens.data(), n_prompt);
    if (llama_decode(ctx, batch) != 0) { llama_free(ctx); return result; }
    double t_prompt = now_ms() - t_start;

    // Set up sampler chain.
    struct llama_sampler_chain_params sparams = llama_sampler_chain_default_params();
    struct llama_sampler *sampler = llama_sampler_chain_init(sparams);
    llama_sampler_chain_add(sampler, llama_sampler_init_top_k(40));
    llama_sampler_chain_add(sampler, llama_sampler_init_top_p(0.9f, 1));
    llama_sampler_chain_add(sampler, llama_sampler_init_temp(0.1f));
    llama_sampler_chain_add(sampler, llama_sampler_init_penalties(64, 1.1f, 0.0f, 0.0f));
    llama_sampler_chain_add(sampler, llama_sampler_init_dist(0xFFFFFFFF));

    // Generate tokens.
    std::string output;
    double t_gen_start = now_ms();
    int post_think_budget = -1; // -1 = not yet seen </think>

    for (int i = 0; i < max_tokens; i++) {
        if (g_abort_flag) break;

        llama_token new_token = llama_sampler_sample(sampler, ctx, -1);

        if (llama_vocab_is_eog(vocab, new_token)) break;

        // Detokenize.
        char piece[256];
        int n = llama_token_to_piece(vocab, new_token, piece, sizeof(piece), 0, true);
        if (n > 0) output.append(piece, n);

        // Early stop after </think> — allow 256 more tokens for the answer.
        if (post_think_budget < 0) {
            if (output.find("</think>") != std::string::npos) {
                post_think_budget = 256;
            }
        } else {
            if (--post_think_budget <= 0) break;
        }

        // Feed token for next iteration.
        batch = llama_batch_get_one(&new_token, 1);
        if (llama_decode(ctx, batch) != 0) break;
    }

    double t_gen = now_ms() - t_gen_start;
    result.text = output;
    result.completion_tokens = (int)(output.size() > 0 ? result.prompt_tokens : 0); // approximate
    // Count actual completion tokens by re-tokenizing output.
    if (!output.empty()) {
        std::vector<llama_token> out_tokens(output.size() + 64);
        int n_out = llama_tokenize(vocab, output.c_str(), (int)output.size(),
                                    out_tokens.data(), (int)out_tokens.size(), false, false);
        if (n_out > 0) result.completion_tokens = n_out;
    }
    if (t_gen > 0) {
        result.tokens_per_second = result.completion_tokens / (t_gen / 1000.0);
    }

    llama_sampler_free(sampler);
    llama_free(ctx);
    return result;
}

// ---------------------------------------------------------------------------
// Daemon mode.
// ---------------------------------------------------------------------------

static int run_daemon(const char *model_path, int n_threads, int gpu_layers,
                      int context_size, int batch_size) {
    // Suppress llama.cpp log spam in daemon mode.
    llama_log_set([](enum ggml_log_level, const char *, void *) {}, nullptr);

    struct llama_model_params mparams = llama_model_default_params();
    mparams.n_gpu_layers = gpu_layers;

    struct llama_model *model = llama_model_load_from_file(model_path, mparams);
    if (!model) {
        fprintf(stdout, "{\"ready\":false,\"error\":\"failed to load model\"}\n");
        fflush(stdout);
        return 1;
    }

    const struct llama_vocab *vocab = llama_model_get_vocab(model);

    // Derive model name from filename.
    std::string model_name = model_path;
    auto slash = model_name.find_last_of("/\\");
    if (slash != std::string::npos) model_name = model_name.substr(slash + 1);
    auto dot = model_name.rfind(".gguf");
    if (dot != std::string::npos) model_name = model_name.substr(0, dot);
    // Lowercase for detection.
    std::string model_lower = str_tolower(model_name);

    fprintf(stdout, "{\"ready\":true}\n");
    fflush(stdout);

    char line[65536];
    while (fgets(line, sizeof(line), stdin)) {
        std::string cmd(line);
        g_abort_flag = 0;

        if (json_has_key(cmd, "quit")) break;

        // Parse messages.
        auto messages = parse_messages(cmd);
        if (messages.empty()) {
            fprintf(stdout, "{\"error\":\"no messages\"}\n");
            fflush(stdout);
            continue;
        }

        // Extract system + user messages.
        std::string system_msg, user_msg;
        for (auto &m : messages) {
            if (m.role == "system") system_msg = m.content;
            else if (m.role == "user") user_msg = m.content;
        }

        int max_tokens = json_get_int(cmd, "max_tokens", 512);

        // Thinking model handling.
        bool thinking = is_thinking_model(model_lower);
        bool qwen35 = is_qwen35(model_lower);

        if (thinking && !qwen35 && !system_msg.empty()) {
            system_msg = "/no_think\n" + system_msg;
        }

        // Dynamic token cap.
        int input_words = ((int)system_msg.size() + (int)user_msg.size()) / 5;
        if (thinking) {
            int cap = input_words * 5 + 512;
            if (cap < 2048) cap = 2048;
            if (max_tokens > cap) max_tokens = cap;
        } else {
            int cap = input_words * 3 + 128;
            if (cap < 512) cap = 512;
            if (max_tokens > cap) max_tokens = cap;
        }

        // Apply chat template.
        struct llama_chat_message chat_msgs[2] = {
            { "system", system_msg.c_str() },
            { "user",   user_msg.c_str()   },
        };
        int n_msgs = system_msg.empty() ? 1 : 2;
        struct llama_chat_message *msgs_ptr = system_msg.empty() ? &chat_msgs[1] : &chat_msgs[0];

        // Measure first, then format.
        int tmpl_len = llama_chat_apply_template(llama_model_chat_template(model, nullptr),
                                                  msgs_ptr, n_msgs, true, nullptr, 0);
        if (tmpl_len < 0) {
            fprintf(stdout, "{\"error\":\"chat template failed\"}\n");
            fflush(stdout);
            continue;
        }
        std::vector<char> prompt_buf(tmpl_len + 1);
        llama_chat_apply_template(llama_model_chat_template(model, nullptr),
                                   msgs_ptr, n_msgs, true, prompt_buf.data(), (int)prompt_buf.size());
        std::string prompt(prompt_buf.data(), tmpl_len);

        // Qwen3.5 thinking block injection.
        if (qwen35) {
            prompt += "<think>\n\n</think>\n\n";
        }

        // Run inference.
        auto result = complete(model, vocab, prompt, max_tokens, n_threads,
                               context_size, batch_size);

        if (g_abort_flag) {
            fprintf(stdout, "{\"error\":\"aborted\"}\n");
            fflush(stdout);
            continue;
        }

        // Clean output.
        std::string cleaned = clean_response(result.text);

        fprintf(stdout, "{\"text\":\"%s\",\"model\":\"%s\",\"prompt_tokens\":%d,\"completion_tokens\":%d,\"tokens_per_second\":%.1f}\n",
                json_escape(cleaned).c_str(),
                json_escape(model_name).c_str(),
                result.prompt_tokens,
                result.completion_tokens,
                result.tokens_per_second);
        fflush(stdout);
    }

    llama_model_free(model);
    return 0;
}

// ---------------------------------------------------------------------------
// Parent PID watchdog.
// ---------------------------------------------------------------------------

#ifdef _WIN32
static void watch_parent(int parent_pid) {
    HANDLE h = OpenProcess(SYNCHRONIZE, FALSE, (DWORD)parent_pid);
    if (!h) return;
    // Wait in background thread — when parent exits, we exit.
    CreateThread(NULL, 0, [](LPVOID p) -> DWORD {
        WaitForSingleObject((HANDLE)p, INFINITE);
        CloseHandle((HANDLE)p);
        _exit(0);
        return 0;
    }, h, 0, NULL);
}
#else
static void watch_parent(int parent_pid) {
    // Fork a watchdog thread.
    pid_t pid = parent_pid;
    pthread_t t;
    pthread_create(&t, nullptr, [](void *arg) -> void* {
        pid_t ppid = (pid_t)(intptr_t)arg;
        while (kill(ppid, 0) == 0) {
            usleep(500000); // check every 500ms
        }
        _exit(0);
        return nullptr;
    }, (void*)(intptr_t)pid);
    pthread_detach(t);
}
#endif

// ---------------------------------------------------------------------------
// Main.
// ---------------------------------------------------------------------------

int main(int argc, char **argv) {
    const char *model_path = nullptr;
    int n_threads = 4;
    int gpu_layers = 0;
    int context_size = 2048;
    int batch_size = 512;
    int parent_pid = 0;
    bool daemon = false;

    for (int i = 1; i < argc; i++) {
        if (strcmp(argv[i], "-m") == 0 && i + 1 < argc) model_path = argv[++i];
        else if (strcmp(argv[i], "-t") == 0 && i + 1 < argc) n_threads = atoi(argv[++i]);
        else if (strcmp(argv[i], "--gpu-layers") == 0 && i + 1 < argc) gpu_layers = atoi(argv[++i]);
        else if (strcmp(argv[i], "--context-size") == 0 && i + 1 < argc) context_size = atoi(argv[++i]);
        else if (strcmp(argv[i], "--batch-size") == 0 && i + 1 < argc) batch_size = atoi(argv[++i]);
        else if (strcmp(argv[i], "--parent-pid") == 0 && i + 1 < argc) parent_pid = atoi(argv[++i]);
        else if (strcmp(argv[i], "--daemon") == 0) daemon = true;
    }

    if (!model_path || !daemon) {
        fprintf(stderr, "Usage: ghostai --daemon -m <model> [-t <threads>] [--gpu-layers <n>] [--context-size <n>] [--parent-pid <pid>]\n");
        return 2;
    }

    if (parent_pid > 0) {
        watch_parent(parent_pid);
    }

    llama_backend_init();
    int ret = run_daemon(model_path, n_threads, gpu_layers, context_size, batch_size);
    llama_backend_free();
    return ret;
}
