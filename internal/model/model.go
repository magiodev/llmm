package model

import (
	"fmt"
)

// ModelInfo contains information about a model
type ModelInfo struct {
	Key   string
	Name  string
	Alias string
	Path  string
}

// GetModelInfo returns model info for a backend and model key
func GetModelInfo(backend, key string) (*ModelInfo, error) {
	infoMap := map[string]map[string]ModelInfo{
		"turboquant": {
			"35b":    {Key: "35b", Name: "Qwen3.6 35B MoE Q5_K_XL", Alias: "qwen3.6-35b-a3b-moe", Path: "/home/magiodev/LLM/models/gguf/unsloth/Qwen3.6-35B-A3B-GGUF/Qwen3.6-35B-A3B-UD-Q5_K_XL.gguf"},
			"122b":   {Key: "122b", Name: "Qwen3.5 122B A10B Q4_K_M", Alias: "qwen3.5-122b-a10b", Path: "/home/magiodev/LLM/models/gguf/mradermacher/Qwen3.5-122B-A10B-GGUF/Qwen3.5-122B-A10B.Q4_K_M.gguf"},
			"qwopus": {Key: "qwopus", Name: "Qwopus 27B v2 Q6_K", Alias: "qwopus-27b", Path: "/home/magiodev/LLM/models/gguf/Jackrong/Qwopus3.6-27B-v2-MTP-GGUF/Qwopus3.6-27B-v2-MTP-Q6_K.gguf"},
			"nex":    {Key: "nex", Name: "Nex-N2-mini Q4_K_M", Alias: "nex-n2-mini", Path: "/home/magiodev/LLM/models/gguf/bartowski/nex-agi_Nex-N2-mini-GGUF/nex-agi_Nex-N2-mini-Q4_K_M.gguf"},
			"27b":    {Key: "27b", Name: "Qwen3.6 27B Dense Q5_K_XL", Alias: "qwen3.6-27b-dense", Path: "/home/magiodev/LLM/models/gguf/unsloth/Qwen3.6-27B-GGUF/Qwen3.6-27B-UD-Q5_K_XL.gguf"},
			"gemma":  {Key: "gemma", Name: "Gemma 4 12B Coder Q8_0", Alias: "gemma4-12b-coder", Path: "/home/magiodev/LLM/models/gguf/yuxinlu1/gemma-4-12B-coder-fable5-composer2.5-v1/gemma4-coding-Q8_0.gguf"},
		},
		"vllm": {
			"diffusiongemma": {Key: "diffusiongemma", Name: "DiffusionGemma 26B A4B IT", Alias: "diffusiongemma-26b", Path: "/home/magiodev/LLM/models/safetensors/google/diffusiongemma-26B-A4B-IT"},
			"ornith":         {Key: "ornith", Name: "Ornith 1.0 35B MoE FP8", Alias: "ornith-1.0-35b", Path: "/home/magiodev/LLM/models/safetensors/ornith-1.0-35b"},
		},
	}

	backendModels, ok := infoMap[backend]
	if !ok {
		return nil, fmt.Errorf("unknown backend: %s", backend)
	}

	model, ok := backendModels[key]
	if !ok {
		return nil, fmt.Errorf("unknown model: %s for backend %s", key, backend)
	}

	return &model, nil
}
