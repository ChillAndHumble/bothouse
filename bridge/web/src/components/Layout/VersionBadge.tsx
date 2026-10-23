import { useEffect, useRef, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { RefreshCw, ExternalLink, AlertCircle } from 'lucide-react';
import { cn } from '@/lib/utils';
import { getUpdate, applyUpdate, type UpdateInfo } from '@/api/update';
import { getStatus } from '@/api/status';

// Reconnect poll while the program restarts after an update. A restart takes about 10 s
// (bots reconnect in ~10 s, im-bridge/claude/CLAUDE.md), so 2 s keeps the wait short.
const RECONNECT_POLL_MS = 2000;

type Phase = 'idle' | 'updating' | 'restarting' | 'failed';

const v = (s: string) => (s && !s.startsWith('v') && s !== 'dev' ? `v${s}` : s);

export default function VersionBadge({ collapsed }: { collapsed: boolean }) {
  const { t } = useTranslation();
  const [info, setInfo] = useState<UpdateInfo | null>(null);
  const [phase, setPhase] = useState<Phase>('idle');
  const [error, setError] = useState('');
  const [downloadUrl, setDownloadUrl] = useState('');
  const timer = useRef<number | undefined>(undefined);

  useEffect(() => {
    getUpdate().then(setInfo).catch(() => {
      getStatus().then(s => setInfo({ current: s.version, latest: '', has_update: false, dev: false, download_url: '', error: '' })).catch(() => {});
    });
    return () => window.clearInterval(timer.current);
  }, []);

  if (!info) return null;

  const waitForRestart = (from: string) => {
    timer.current = window.setInterval(() => {
      getStatus()
        .then(s => {
          if (s.version && s.version !== from) {
            window.clearInterval(timer.current);
            window.location.reload();
          }
        })
        .catch(() => { /* still restarting */ });
    }, RECONNECT_POLL_MS);
  };

  const onUpdate = async () => {
    setPhase('updating');
    setError('');
    try {
      const r = await applyUpdate();
      if (r.status === 'restarting') {
        setPhase('restarting');
        waitForRestart(info.current);
      } else if (r.status === 'up_to_date') {
        setPhase('idle');
        setInfo({ ...info, has_update: false });
      } else {
        setPhase('failed');
        setError(r.error || '');
        setDownloadUrl(r.download_url || info.download_url);
      }
    } catch (e: any) {
      setPhase('failed');
      setError(e?.message || String(e));
      setDownloadUrl(info.download_url);
    }
  };

  const current = v(info.current);
  const busy = phase === 'updating' || phase === 'restarting';
  const showUpdate = info.has_update && phase !== 'failed';
  const label = phase === 'updating'
    ? t('update.updating', { version: v(info.latest) })
    : phase === 'restarting'
      ? t('update.restarting')
      : t('update.button', { version: v(info.latest) });
  const title = info.dev ? t('update.dev') : info.error ? `${t('update.checkFailed')}: ${info.error}` : undefined;

  if (collapsed) {
    return (
      <div className="flex flex-col items-center gap-1 pb-1" data-testid="version-badge">
        <span className="text-[10px] leading-none text-gray-400 dark:text-gray-500 truncate max-w-full" title={title}>{current}</span>
        {showUpdate && (
          <button
            type="button"
            onClick={onUpdate}
            disabled={busy}
            title={label}
            aria-label={label}
            className="p-1.5 rounded-lg bg-accent/12 text-accent ring-1 ring-accent/25 hover:bg-accent/20 disabled:opacity-60"
          >
            <RefreshCw size={14} className={cn(busy && 'animate-spin')} />
          </button>
        )}
        {phase === 'failed' && (
          <a href={downloadUrl} target="_blank" rel="noopener noreferrer" title={`${t('update.failed')}: ${error}`} className="p-1.5 rounded-lg text-red-500 hover:bg-red-500/10">
            <AlertCircle size={14} />
          </a>
        )}
      </div>
    );
  }

  return (
    <div className="px-2 pb-1 space-y-1.5" data-testid="version-badge">
      <div className="text-xs text-gray-400 dark:text-gray-500 select-none" title={title}>
        bothouse {current}
        {info.dev && <span className="ml-1">· {t('update.devShort')}</span>}
      </div>
      {showUpdate && (
        <button
          type="button"
          onClick={onUpdate}
          disabled={busy}
          className={cn(
            'flex items-center justify-center gap-1.5 w-full px-2 py-1.5 rounded-lg text-xs font-medium transition-colors',
            'bg-accent/12 text-accent ring-1 ring-accent/25 hover:bg-accent/20 disabled:opacity-70',
          )}
        >
          <RefreshCw size={13} className={cn(busy && 'animate-spin')} />
          <span className="truncate">{label}</span>
        </button>
      )}
      {phase === 'failed' && (
        <div className="rounded-lg bg-red-500/10 px-2 py-1.5 text-xs text-red-600 dark:text-red-400 space-y-1">
          <div className="flex items-start gap-1">
            <AlertCircle size={13} className="shrink-0 mt-px" />
            <span className="min-w-0 [overflow-wrap:anywhere]">{t('update.failed')}{error ? `: ${error}` : ''}</span>
          </div>
          <a href={downloadUrl} target="_blank" rel="noopener noreferrer" className="inline-flex items-center gap-1 font-medium underline">
            <ExternalLink size={12} />
            {t('update.openDownload')}
          </a>
        </div>
      )}
    </div>
  );
}
