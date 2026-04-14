package i18n

import (
	"fmt"
	"os"
	"strings"
)

var defaultLang = "pt"
var currentLang = "pt"

var messages = map[string]map[string]string{
	"pt": {
		"tool_websearch_description":       "Realiza uma busca na internet usando o DuckDuckGo e retorna a resposta direta, definição, resumo e os principais resultados relacionados. Use para obter informações atuais, definições, documentação ou qualquer dado que não esteja disponível localmente.",
		"tool_websearch_query_description": "A consulta de busca. Seja preciso e específico — use termos técnicos quando apropriado (ex: 'golang http middleware pattern', 'AES-GCM encryption Go example').",
		"tool_websearch_header":            "# Resultados para: %q\n\n",
		"tool_websearch_direct_answer":     "## Resposta Direta\n",
		"tool_websearch_definition":        "## Definição\n",
		"tool_websearch_definition_source": "Fonte: %s (%s)\n",
		"tool_websearch_abstract":          "## Resumo\n",
		"tool_websearch_abstract_source":   "Fonte: %s (%s)\n",
		"tool_websearch_results":           "## Resultados\n",
		"tool_websearch_related":           "## Tópicos Relacionados\n",
		"tool_websearch_no_results":        "Nenhum resultado encontrado para %q. Tente reformular a busca com termos mais específicos.",
		"wizard_title":               "🍯 Configuração Inicial 🍯",
		"wizard_subtitle":            "Primeiro voo da Colmeia Jandaira",
		"wizard_err_save":            "Erro ao salvar configuração: ",
		"wizard_success":             "✅ Configuração concluída!",
		"wizard_system_msg":          "A colmeia precisa de algumas definições antes de acordar.",
		"wizard_footer":              "↵ confirmar • esc sair (deixe em branco para o padrão)",
		"wizard_prompt_lang":         "1. Idioma (pt/en/es/ru/zh) ",
		"wizard_prompt_api_key":      "2. Chave da API OpenAI (sk-...) ",
		"wizard_place_api_key":       "Se vazio, tenta ambiente...",
		"wizard_prompt_save_path":    "3. Salvar configuração em ",
		"wizard_prompt_model":        "4. Modelo OpenAI ",
		"wizard_prompt_swarm":        "5. Nome do Enxame ",
		"wizard_prompt_nectar":       "6. Limite de Néctar (Tokens) ",
		"wizard_prompt_max_agents":   "7. Número máximo de agentes ",
		"wizard_prompt_supervised":   "8. Modo Supervisionado? (S/n) ",
		"wizard_prompt_isolated":     "9. Modo Isolado / Sandbox Wasm? (S/n) ",
		"cli_header_title":           "🍯  Jandaira Swarm OS  🍯",
		"cli_header_subtitle":        "Swarm Intelligence · Powered by Go",
		"cli_greeting":               "✦ A Colmeia Jandaira despertou. As operárias aguardam as suas ordens.",
		"cli_prompt_goal":            "🐝 Objetivo",
		"cli_prompt_placeholder":     "Diga à Rainha o que...",
		"cli_footer":                 "↵ enviar   esc / ctrl+c sair",
		"cli_msg_processing":         "A Rainha está pensando...",
		"cli_msg_done":               "O Workflow terminou!",
		"cli_agent_delegating":       "Delegando para o agente...",
		"cli_request_approval":       "A IA quer usar a ferramenta",
		"cli_approval_prompt":        "👨‍🌾 Você autoriza? (S = sim / N = não)",
		"warn_api_key_not_set":       "⚠️ Aviso: OPENAI_API_KEY não definida no cofre nem no ambiente.",
		"cli_api_init_error":         "Erro no servidor da api: %v",
	},
	"en": {
		"tool_websearch_description":       "Performs an internet search using DuckDuckGo and returns the direct answer, definition, summary, and top related results. Use to get current information, definitions, documentation, or any data not available locally.",
		"tool_websearch_query_description": "The search query. Be precise and specific — use technical terms when appropriate (e.g. 'golang http middleware pattern', 'AES-GCM encryption Go example').",
		"tool_websearch_header":            "# Results for: %q\n\n",
		"tool_websearch_direct_answer":     "## Direct Answer\n",
		"tool_websearch_definition":        "## Definition\n",
		"tool_websearch_definition_source": "Source: %s (%s)\n",
		"tool_websearch_abstract":          "## Summary\n",
		"tool_websearch_abstract_source":   "Source: %s (%s)\n",
		"tool_websearch_results":           "## Results\n",
		"tool_websearch_related":           "## Related Topics\n",
		"tool_websearch_no_results":        "No results found for %q. Try rephrasing the search with more specific terms.",
		"wizard_title":               "🍯 Initial Setup 🍯",
		"wizard_subtitle":            "First flight of the Jandaira Swarm",
		"wizard_err_save":            "Error saving configuration: ",
		"wizard_success":             "✅ Configuration complete!",
		"wizard_system_msg":          "The hive needs a few settings before waking up.",
		"wizard_footer":              "↵ confirm • esc quit (leave blank for default)",
		"wizard_prompt_lang":         "1. Language (pt/en/es/ru/zh) ",
		"wizard_prompt_api_key":      "2. OpenAI API Key (sk-...) ",
		"wizard_place_api_key":       "If empty, tries environment...",
		"wizard_prompt_save_path":    "3. Save configuration in ",
		"wizard_prompt_model":        "4. OpenAI Model ",
		"wizard_prompt_swarm":        "5. Swarm Name ",
		"wizard_prompt_nectar":       "6. Nectar Limit (Tokens) ",
		"wizard_prompt_max_agents":   "7. Maximum number of agents ",
		"wizard_prompt_supervised":   "8. Supervised Mode? (Y/n) ",
		"wizard_prompt_isolated":     "9. Isolated Mode / Wasm Sandbox? (Y/n) ",
		"cli_header_title":           "🍯  Jandaira Swarm OS  🍯",
		"cli_header_subtitle":        "Swarm Intelligence · Powered by Go",
		"cli_greeting":               "✦ The Jandaira Hive has awakened. The workers await your orders.",
		"cli_prompt_goal":            "🐝 Goal",
		"cli_prompt_placeholder":     "Tell the Queen what...",
		"cli_footer":                 "↵ submit   esc / ctrl+c quit",
		"cli_msg_processing":         "The Queen is thinking...",
		"cli_msg_done":               "Workflow finished!",
		"cli_agent_delegating":       "Delegating to agent...",
		"cli_request_approval":       "The AI wants to use the tool",
		"cli_approval_prompt":        "👨‍🌾 Do you authorize? (Y = yes / N = no)",
		"warn_api_key_not_set":       "⚠️ Warning: OPENAI_API_KEY is not set in vault or env.",
		"cli_api_init_error":         "API server error: %v",
	},
	"es": {
		"tool_websearch_description":       "Realiza una búsqueda en internet usando DuckDuckGo y devuelve la respuesta directa, definición, resumen y los principales resultados relacionados. Úselo para obtener información actual, definiciones, documentación o cualquier dato no disponible localmente.",
		"tool_websearch_query_description": "La consulta de búsqueda. Sea preciso y específico — use términos técnicos cuando sea apropiado (ej: 'golang http middleware pattern', 'AES-GCM encryption Go example').",
		"tool_websearch_header":            "# Resultados para: %q\n\n",
		"tool_websearch_direct_answer":     "## Respuesta Directa\n",
		"tool_websearch_definition":        "## Definición\n",
		"tool_websearch_definition_source": "Fuente: %s (%s)\n",
		"tool_websearch_abstract":          "## Resumen\n",
		"tool_websearch_abstract_source":   "Fuente: %s (%s)\n",
		"tool_websearch_results":           "## Resultados\n",
		"tool_websearch_related":           "## Temas Relacionados\n",
		"tool_websearch_no_results":        "No se encontraron resultados para %q. Intente reformular la búsqueda con términos más específicos.",
		"wizard_title":               "🍯 Configuración Inicial 🍯",
		"wizard_subtitle":            "Primer vuelo de la Colmena Jandaira",
		"wizard_err_save":            "Error al guardar configuración: ",
		"wizard_success":             "✅ ¡Configuración completada!",
		"wizard_system_msg":          "La colmena necesita algunas definiciones antes de despertar.",
		"wizard_footer":              "↵ confirmar • esc salir (dejar en blanco para predeterminado)",
		"wizard_prompt_lang":         "1. Idioma (pt/en/es/ru/zh) ",
		"wizard_prompt_api_key":      "2. Clave de la API OpenAI (sk-...) ",
		"wizard_place_api_key":       "Si vacío, intenta con entorno...",
		"wizard_prompt_save_path":    "3. Guardar configuración en ",
		"wizard_prompt_model":        "4. Modelo OpenAI ",
		"wizard_prompt_swarm":        "5. Nombre del Enjambre ",
		"wizard_prompt_nectar":       "6. Límite de Néctar (Tokens) ",
		"wizard_prompt_max_agents":   "7. Número máximo de agentes ",
		"wizard_prompt_supervised":   "8. ¿Modo Supervisado? (S/n) ",
		"wizard_prompt_isolated":     "9. ¿Modo Aislado / Sandbox Wasm? (S/n) ",
		"cli_header_title":           "🍯  Jandaira Swarm OS  🍯",
		"cli_header_subtitle":        "Swarm Intelligence · Powered by Go",
		"cli_greeting":               "✦ La Colmena Jandaira ha despertado. Las obreras esperan sus órdenes.",
		"cli_prompt_goal":            "🐝 Objetivo",
		"cli_prompt_placeholder":     "Diga a la Reina qué...",
		"cli_footer":                 "↵ enviar   esc / ctrl+c salir",
		"cli_msg_processing":         "La Reina está pensando...",
		"cli_msg_done":               "¡El Workflow ha terminado!",
		"cli_agent_delegating":       "Delegando al agente...",
		"cli_request_approval":       "La IA quiere usar la herramienta",
		"cli_approval_prompt":        "👨‍🌾 ¿Usted autoriza? (S = sí / N = no)",
		"warn_api_key_not_set":       "⚠️ Aviso: OPENAI_API_KEY no definida ni en la bóveda ni en entorno.",
		"cli_api_init_error":         "Error en el servidor de la API: %v",
	},
	"ru": {
		"tool_websearch_description":       "Выполняет поиск в интернете с помощью DuckDuckGo и возвращает прямой ответ, определение, резюме и основные связанные результаты. Используйте для получения актуальной информации, определений, документации или данных, недоступных локально.",
		"tool_websearch_query_description": "Поисковый запрос. Будьте точны и конкретны — используйте технические термины там, где это уместно (напр.: 'golang http middleware pattern', 'AES-GCM encryption Go example').",
		"tool_websearch_header":            "# Результаты для: %q\n\n",
		"tool_websearch_direct_answer":     "## Прямой Ответ\n",
		"tool_websearch_definition":        "## Определение\n",
		"tool_websearch_definition_source": "Источник: %s (%s)\n",
		"tool_websearch_abstract":          "## Резюме\n",
		"tool_websearch_abstract_source":   "Источник: %s (%s)\n",
		"tool_websearch_results":           "## Результаты\n",
		"tool_websearch_related":           "## Связанные Темы\n",
		"tool_websearch_no_results":        "Результаты для %q не найдены. Попробуйте переформулировать запрос с более конкретными терминами.",
		"wizard_title":               "🍯 Начальная настройка 🍯",
		"wizard_subtitle":            "Первый полет роя Жандаира",
		"wizard_err_save":            "Ошибка сохранения конфигурации: ",
		"wizard_success":             "✅ Настройка завершена!",
		"wizard_system_msg":          "Улью нужно задать некоторые настройки перед пробуждением.",
		"wizard_footer":              "↵ подтвердить • esc выход (оставьте пустым для умолчания)",
		"wizard_prompt_lang":         "1. Язык (pt/en/es/ru/zh) ",
		"wizard_prompt_api_key":      "2. Ключ API OpenAI (sk-...) ",
		"wizard_place_api_key":       "Если пусто, пытается взять из среды...",
		"wizard_prompt_save_path":    "3. Сохранить конфигурацию в ",
		"wizard_prompt_model":        "4. Модель OpenAI ",
		"wizard_prompt_swarm":        "5. Имя роя ",
		"wizard_prompt_nectar":       "6. Лимит нектара (токены) ",
		"wizard_prompt_max_agents":   "7. Максимальное количество агентов ",
		"wizard_prompt_supervised":   "8. Обязательный контроль? (Y/n) ",
		"wizard_prompt_isolated":     "9. Изолированный режим (Wasm Sandbox)? (Y/n) ",
		"cli_header_title":           "🍯  Jandaira Swarm OS  🍯",
		"cli_header_subtitle":        "Роевой интеллект · Работает на Go",
		"cli_greeting":               "✦ Улей пробудился. Рабочие ожидают ваших приказов.",
		"cli_prompt_goal":            "🐝 Цель",
		"cli_prompt_placeholder":     "Скажите Королеве, что...",
		"cli_footer":                 "↵ отправить   esc / ctrl+c выход",
		"cli_msg_processing":         "Королева думает...",
		"cli_msg_done":               "Рабочий процесс завершен!",
		"cli_agent_delegating":       "Делегировано агенту...",
		"cli_request_approval":       "ИИ хочет использовать инструмент",
		"cli_approval_prompt":        "👨‍🌾 Вы даете разрешение? (Y = да / N = нет)",
		"warn_api_key_not_set":       "⚠️ Предупреждение: ключ OPENAI_API_KEY не установлен.",
		"cli_api_init_error":         "Ошибка сервера API: %v",
	},
	"zh": {
		"tool_websearch_description":       "使用DuckDuckGo在网络上进行搜索，返回直接答案、定义、摘要及主要相关结果。用于获取最新信息、定义、文档或本地无法获取的数据。",
		"tool_websearch_query_description": "搜索查询。请精确且具体 — 适当时使用技术术语（例如：'golang http middleware pattern'、'AES-GCM encryption Go example'）。",
		"tool_websearch_header":            "# 搜索结果: %q\n\n",
		"tool_websearch_direct_answer":     "## 直接回答\n",
		"tool_websearch_definition":        "## 定义\n",
		"tool_websearch_definition_source": "来源: %s (%s)\n",
		"tool_websearch_abstract":          "## 摘要\n",
		"tool_websearch_abstract_source":   "来源: %s (%s)\n",
		"tool_websearch_results":           "## 结果\n",
		"tool_websearch_related":           "## 相关主题\n",
		"tool_websearch_no_results":        "未找到 %q 的相关结果。请尝试用更具体的词语重新搜索。",
		"wizard_title":               "🍯 初始设置 🍯",
		"wizard_subtitle":            "Jandaira蜂巢的首次飞行",
		"wizard_err_save":            "保存配置时出错: ",
		"wizard_success":             "✅ 配置完成！",
		"wizard_system_msg":          "蜂巢在醒来前需要一些定义。",
		"wizard_footer":              "↵ 确认 • esc 退出 (留空使用默认值)",
		"wizard_prompt_lang":         "1. 语言 (pt/en/es/ru/zh) ",
		"wizard_prompt_api_key":      "2. OpenAI API 密钥 (sk-...) ",
		"wizard_place_api_key":       "如果为空，尝试环境变量...",
		"wizard_prompt_save_path":    "3. 保存配置于 ",
		"wizard_prompt_model":        "4. OpenAI 模型 ",
		"wizard_prompt_swarm":        "5. 蜂群名称 ",
		"wizard_prompt_nectar":       "6. 花蜜限制(Tokens) ",
		"wizard_prompt_max_agents":   "7. 最大代理数量 ",
		"wizard_prompt_supervised":   "8. 监督模式? (Y/n) ",
		"wizard_prompt_isolated":     "9. 隔离模式 (Wasm Sandbox)? (Y/n) ",
		"cli_header_title":           "🍯  Jandaira Swarm OS  🍯",
		"cli_header_subtitle":        "集群智能 · Go语言强力驱动",
		"cli_greeting":               "✦ 蜂巢已苏醒。工蜂们正在等待您的命令。",
		"cli_prompt_goal":            "🐝 目标",
		"cli_prompt_placeholder":     "告诉女王该做什么...",
		"cli_footer":                 "↵ 提交   esc / ctrl+c 退出",
		"cli_msg_processing":         "女王正在思考...",
		"cli_msg_done":               "工作流结束！",
		"cli_agent_delegating":       "委派给代理...",
		"cli_request_approval":       "AI想使用工具",
		"cli_approval_prompt":        "👨‍🌾 您是否授权？ (Y = 是 / N = 否)",
		"warn_api_key_not_set":       "⚠️ 警告: 未设置 OPENAI_API_KEY。",
		"cli_api_init_error":         "API 服务器错误: %v",
	},
}

// Init define o idioma base com o LANG do SO local se não for explicitamente invocado.
func Init() {
	lang := os.Getenv("JANDAIRA_LANG")
	if lang == "" {
		lang = os.Getenv("LANG")
	}
	SetLanguage(lang)
}

// SetLanguage muda o idioma do sistema atual.
func SetLanguage(lang string) {
	lang = strings.ToLower(lang)
	if strings.HasPrefix(lang, "en") {
		currentLang = "en"
	} else if strings.HasPrefix(lang, "es") {
		currentLang = "es"
	} else if strings.HasPrefix(lang, "ru") {
		currentLang = "ru"
	} else if strings.HasPrefix(lang, "zh") {
		currentLang = "zh"
	} else {
		currentLang = "pt"
	}
}

// CurrentLang devolve a identificação do idioma a ser utilizado
func CurrentLang() string {
	return currentLang
}

// T devolve o texto traduzido formatando-o caso possua argumentos extra
func T(key string, args ...interface{}) string {
	dict, ok := messages[currentLang]
	if !ok {
		dict = messages[defaultLang]
	}

	val, ok := dict[key]
	if !ok {
		// Fallback to PT if key not found
		val = messages[defaultLang][key]
	}

	if len(args) > 0 {
		return fmt.Sprintf(val, args...)
	}
	return val
}
