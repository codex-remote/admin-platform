UPDATE events
SET fields_json = COALESCE((
  SELECT jsonb_object_agg(entry.key, entry.value)
  FROM jsonb_each(events.fields_json) AS entry
  WHERE entry.key = ANY (ARRAY[
    'attempt','available_mb','bytes','cached_transcripts','connection_id',
    'console_entries','count','delay_ms','dropped_count','dropped_events',
    'duration_ms','item_type','live_characters','live_items','message_type',
    'outcome','phase','pid','project_count','projects','prompt_characters',
    'reason','relay_mb','relay_messages','role','status','thread_count',
    'threads','transcript_messages','transport'
  ])
), '{}'::jsonb);
