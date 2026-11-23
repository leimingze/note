-- 功能：导出 v2 E4 最终一轮并发门控的逐任务记录。
-- 启动命令：mysql -NB --raw pulseframe_review_v3 < export_p1.sql > p1_gate.jsonl
-- 该轮从 36588531 开始，后续 40 条为跳过门控的直审任务。
SELECT JSON_OBJECT(
    'task_id', task_id,
    'media_key', media_key,
    'status', status,
    'result', result,
    'risk_score', result_json->'$.gate.risk_score',
    'threshold', result_json->'$.gate.threshold',
    'early_exit', result_json->'$.gate.early_exit',
    'hard_bypass', result_json->'$.gate.hard_bypass',
    'bypass_reasons', COALESCE(result_json->'$.gate.bypass_reasons', JSON_ARRAY())
)
FROM task
WHERE task_id BETWEEN 36588531 AND 36588570 AND kind = 'publish'
ORDER BY task_id;
