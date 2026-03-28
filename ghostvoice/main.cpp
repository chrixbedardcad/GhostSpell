// ghostvoice — GhostSpell speech-to-text helper.
// Minimal C++ binary that links whisper.cpp directly (no CGo).
//
// Single-shot mode:
//   ghostvoice -m <model> -f <wav> [-l <lang>] [-t <threads>]
//
// Daemon mode (persistent process, model stays loaded):
//   ghostvoice --daemon -m <model> [-t <threads>]
//   Then send JSON commands on stdin, one per line:
//     {"file":"path.wav","lang":"en"}
//   Receive JSON responses on stdout, one per line:
//     {"text":"transcribed text"}
//   Send {"quit":true} or close stdin to exit.

#include "whisper.h"

#include <cstdio>
#include <cstdlib>
#include <cstring>
#include <string>
#include <vector>

static bool read_wav_to_float(const char *path, std::vector<float> &out, int target_rate = 16000) {
    FILE *f = fopen(path, "rb");
    if (!f) return false;

    char riff[4]; fread(riff, 1, 4, f);
    if (memcmp(riff, "RIFF", 4) != 0) { fclose(f); return false; }
    fseek(f, 4, SEEK_CUR);
    char wave[4]; fread(wave, 1, 4, f);
    if (memcmp(wave, "WAVE", 4) != 0) { fclose(f); return false; }

    int channels = 1, sample_rate = 16000, bits_per_sample = 16;
    std::vector<uint8_t> pcm_data;

    while (!feof(f)) {
        char id[4]; uint32_t size;
        if (fread(id, 1, 4, f) != 4) break;
        if (fread(&size, 4, 1, f) != 1) break;

        if (memcmp(id, "fmt ", 4) == 0) {
            uint16_t fmt, ch; uint32_t sr; uint16_t bps;
            fread(&fmt, 2, 1, f); fread(&ch, 2, 1, f);
            fread(&sr, 4, 1, f); fseek(f, 6, SEEK_CUR);
            fread(&bps, 2, 1, f);
            channels = ch; sample_rate = sr; bits_per_sample = bps;
            if (size > 16) fseek(f, size - 16, SEEK_CUR);
        } else if (memcmp(id, "data", 4) == 0) {
            pcm_data.resize(size);
            fread(pcm_data.data(), 1, size, f);
        } else {
            fseek(f, size, SEEK_CUR);
        }
        if (size % 2 != 0) fseek(f, 1, SEEK_CUR);
    }
    fclose(f);

    if (pcm_data.empty() || bits_per_sample != 16) return false;

    int bytes_per_sample = bits_per_sample / 8;
    int stride = channels * bytes_per_sample;
    int n_frames = (int)pcm_data.size() / stride;

    std::vector<float> mono(n_frames);
    for (int i = 0; i < n_frames; i++) {
        double sum = 0;
        for (int ch = 0; ch < channels; ch++) {
            int pos = i * stride + ch * bytes_per_sample;
            int16_t s = (int16_t)(pcm_data[pos] | (pcm_data[pos + 1] << 8));
            sum += s;
        }
        mono[i] = (float)(sum / channels / 32768.0);
    }

    if (sample_rate != target_rate) {
        double ratio = (double)sample_rate / target_rate;
        int out_len = (int)(n_frames / ratio);
        out.resize(out_len);
        for (int i = 0; i < out_len; i++) {
            double src = i * ratio;
            int idx = (int)src;
            if (idx >= n_frames - 1) idx = n_frames - 2;
            float frac = (float)(src - idx);
            out[i] = mono[idx] * (1.0f - frac) + mono[idx + 1] * frac;
        }
    } else {
        out = std::move(mono);
    }

    return true;
}

// Simple JSON helpers (no library needed for our fixed format).
static std::string json_get_string(const std::string &json, const char *key) {
    std::string needle = std::string("\"") + key + "\":\"";
    auto pos = json.find(needle);
    if (pos == std::string::npos) return "";
    pos += needle.size();
    auto end = json.find("\"", pos);
    if (end == std::string::npos) return "";
    // Handle escaped characters in file paths.
    std::string result;
    for (size_t i = pos; i < end; i++) {
        if (json[i] == '\\' && i + 1 < end) {
            char next = json[i + 1];
            if (next == '\\') { result += '\\'; i++; }
            else if (next == '"') { result += '"'; i++; }
            else if (next == 'n') { result += '\n'; i++; }
            else if (next == '/') { result += '/'; i++; }
            else result += json[i];
        } else {
            result += json[i];
        }
    }
    return result;
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

static std::string transcribe(struct whisper_context *ctx, const char *wav_path,
                               const char *language, int n_threads) {
    std::vector<float> pcm;
    if (!read_wav_to_float(wav_path, pcm)) {
        return "";
    }

    struct whisper_full_params params = whisper_full_default_params(WHISPER_SAMPLING_GREEDY);
    params.n_threads = n_threads;
    params.print_progress = false;
    params.print_realtime = false;
    params.print_timestamps = false;
    params.print_special = false;
    params.no_timestamps = true;
    params.single_segment = false;
    params.language = language;
    params.detect_language = (strcmp(language, "auto") == 0);

    int ret = whisper_full(ctx, params, pcm.data(), (int)pcm.size());
    if (ret != 0) return "";

    std::string text;
    int n = whisper_full_n_segments(ctx);
    for (int i = 0; i < n; i++) {
        const char *t = whisper_full_get_segment_text(ctx, i);
        if (t) text += t;
    }
    return text;
}

static int run_daemon(const char *model_path, int n_threads) {
    // Suppress whisper.cpp log spam in daemon mode.
    whisper_log_set([](enum ggml_log_level, const char *, void *) {}, nullptr);

    struct whisper_context_params cparams = whisper_context_default_params();
    cparams.flash_attn = true;
    struct whisper_context *ctx = whisper_init_from_file_with_params(model_path, cparams);
    if (!ctx) {
        fprintf(stdout, "{\"ready\":false,\"error\":\"failed to load model\"}\n");
        fflush(stdout);
        return 1;
    }

    fprintf(stdout, "{\"ready\":true}\n");
    fflush(stdout);

    char line[8192];
    while (fgets(line, sizeof(line), stdin)) {
        std::string cmd(line);

        if (json_has_key(cmd, "quit")) break;

        std::string wav_path = json_get_string(cmd, "file");
        std::string lang = json_get_string(cmd, "lang");
        if (lang.empty()) lang = "en";

        if (wav_path.empty()) {
            fprintf(stdout, "{\"error\":\"missing file\"}\n");
            fflush(stdout);
            continue;
        }

        std::string text = transcribe(ctx, wav_path.c_str(), lang.c_str(), n_threads);
        if (text.empty()) {
            fprintf(stdout, "{\"error\":\"transcription failed\"}\n");
        } else {
            fprintf(stdout, "{\"text\":\"%s\"}\n", json_escape(text).c_str());
        }
        fflush(stdout);
    }

    whisper_free(ctx);
    return 0;
}

int main(int argc, char **argv) {
    const char *model_path = nullptr;
    const char *wav_path = nullptr;
    const char *language = "en";
    int n_threads = 4;
    bool daemon = false;

    for (int i = 1; i < argc; i++) {
        if (strcmp(argv[i], "-m") == 0 && i + 1 < argc) model_path = argv[++i];
        else if (strcmp(argv[i], "-f") == 0 && i + 1 < argc) wav_path = argv[++i];
        else if (strcmp(argv[i], "-l") == 0 && i + 1 < argc) language = argv[++i];
        else if (strcmp(argv[i], "-t") == 0 && i + 1 < argc) n_threads = atoi(argv[++i]);
        else if (strcmp(argv[i], "--daemon") == 0) daemon = true;
    }

    if (daemon) {
        if (!model_path) {
            fprintf(stderr, "Usage: ghostvoice --daemon -m <model> [-t <threads>]\n");
            return 2;
        }
        return run_daemon(model_path, n_threads);
    }

    // Single-shot mode.
    if (!model_path || !wav_path) {
        fprintf(stderr, "Usage: ghostvoice -m <model> -f <wav> [-l <lang>] [-t <threads>]\n");
        return 2;
    }

    std::vector<float> pcm;
    if (!read_wav_to_float(wav_path, pcm)) {
        fprintf(stderr, "ghostvoice: failed to read WAV: %s\n", wav_path);
        return 1;
    }
    fprintf(stderr, "ghostvoice: %d samples (%.1fs)\n", (int)pcm.size(), pcm.size() / 16000.0);

    struct whisper_context_params cparams = whisper_context_default_params();
    cparams.flash_attn = true;
    struct whisper_context *ctx = whisper_init_from_file_with_params(model_path, cparams);
    if (!ctx) {
        fprintf(stderr, "ghostvoice: failed to load model: %s\n", model_path);
        return 1;
    }

    std::string text = transcribe(ctx, wav_path, language, n_threads);
    if (!text.empty()) fprintf(stdout, "%s", text.c_str());

    whisper_free(ctx);
    return 0;
}
