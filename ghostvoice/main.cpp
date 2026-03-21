// ghostvoice — GhostSpell speech-to-text helper.
// Minimal C++ binary that links whisper.cpp directly (no CGo).
// Usage: ghostvoice -m <model> -f <wav> [-l <lang>] [-t <threads>]
// Output: transcribed text on stdout.

#include "whisper.h"

#include <cstdio>
#include <cstdlib>
#include <cstring>
#include <string>
#include <vector>

static bool read_wav_to_float(const char *path, std::vector<float> &out, int target_rate = 16000) {
    FILE *f = fopen(path, "rb");
    if (!f) return false;

    // Read RIFF header.
    char riff[4]; fread(riff, 1, 4, f);
    if (memcmp(riff, "RIFF", 4) != 0) { fclose(f); return false; }
    fseek(f, 4, SEEK_CUR); // skip file size
    char wave[4]; fread(wave, 1, 4, f);
    if (memcmp(wave, "WAVE", 4) != 0) { fclose(f); return false; }

    int channels = 1, sample_rate = 16000, bits_per_sample = 16;
    std::vector<uint8_t> pcm_data;

    // Parse chunks.
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
        // Chunks are 2-byte aligned.
        if (size % 2 != 0) fseek(f, 1, SEEK_CUR);
    }
    fclose(f);

    if (pcm_data.empty() || bits_per_sample != 16) return false;

    // Convert to float32 mono.
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

    // Resample to target rate if needed.
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

int main(int argc, char **argv) {
    const char *model_path = nullptr;
    const char *wav_path = nullptr;
    const char *language = "en";
    int n_threads = 4;

    for (int i = 1; i < argc; i++) {
        if (strcmp(argv[i], "-m") == 0 && i + 1 < argc) model_path = argv[++i];
        else if (strcmp(argv[i], "-f") == 0 && i + 1 < argc) wav_path = argv[++i];
        else if (strcmp(argv[i], "-l") == 0 && i + 1 < argc) language = argv[++i];
        else if (strcmp(argv[i], "-t") == 0 && i + 1 < argc) n_threads = atoi(argv[++i]);
    }

    if (!model_path || !wav_path) {
        fprintf(stderr, "Usage: ghostvoice -m <model> -f <wav> [-l <lang>] [-t <threads>]\n");
        return 2;
    }

    // Read audio.
    std::vector<float> pcm;
    if (!read_wav_to_float(wav_path, pcm)) {
        fprintf(stderr, "ghostvoice: failed to read WAV: %s\n", wav_path);
        return 1;
    }
    fprintf(stderr, "ghostvoice: %d samples (%.1fs)\n", (int)pcm.size(), pcm.size() / 16000.0);

    // Load model.
    struct whisper_context_params cparams = whisper_context_default_params();
    struct whisper_context *ctx = whisper_init_from_file_with_params(model_path, cparams);
    if (!ctx) {
        fprintf(stderr, "ghostvoice: failed to load model: %s\n", model_path);
        return 1;
    }

    // Transcribe.
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
    if (ret != 0) {
        fprintf(stderr, "ghostvoice: whisper_full failed (ret=%d)\n", ret);
        whisper_free(ctx);
        return 1;
    }

    // Output text.
    int n_segments = whisper_full_n_segments(ctx);
    for (int i = 0; i < n_segments; i++) {
        const char *text = whisper_full_get_segment_text(ctx, i);
        if (text) fprintf(stdout, "%s", text);
    }

    whisper_free(ctx);
    return 0;
}
