import { useEffect, useState } from 'react';
import { Button, Card } from '@/components/ui';
import { getDSHStatus, installDSHPlugin, type DSHStatus } from '@/api/wizard';

export default function DSHSetup({ lang, onBack }: { lang: string; onBack: () => void }) {
  const zh = lang.toLowerCase().startsWith('zh');
  const ja = lang.toLowerCase().startsWith('ja');
  const t = zh ? {
    title: 'DSH 安装', steps: ['检测 dsh', '安装插件', '打开 DSH 网页'],
    note: '首次安装完成后，机器人、CLI、人设和模型都在 DSH 页面上配置。',
    detect: '重新检测', missing: '未找到 dsh，请先运行 npm i -g @deepseek-ai/dsh，再重新检测。',
    install: '安装插件', busy: '处理中…', open: '在终端运行下面的命令，打开 DSH 网页：', back: '返回选择平台',
  } : ja ? {
    title: 'DSH セットアップ', steps: ['dsh を検出', 'プラグインをインストール', 'DSH ウェブを開く'],
    note: '初回インストール後、ボット、CLI、ペルソナ、モデルは DSH ページで設定します。',
    detect: '再検出', missing: 'dsh がありません。npm i -g @deepseek-ai/dsh を実行し、再検出してください。',
    install: 'プラグインをインストール', busy: '処理中…', open: 'ターミナルで次のコマンドを実行し、DSH ウェブを開いてください：', back: 'プラットフォーム選択に戻る',
  } : {
    title: 'DSH setup', steps: ['Detect dsh', 'Install plugins', 'Open DSH web'],
    note: 'After this first installation, configure bots, CLIs, personas and models on the DSH page.',
    detect: 'Detect again', missing: 'dsh not found. Run npm i -g @deepseek-ai/dsh, then detect again.',
    install: 'Install plugins', busy: 'Working…', open: 'Run this command in a terminal to open DSH web:', back: 'Back to platforms',
  };
  const [status, setStatus] = useState<DSHStatus | null>(null);
  const [step, setStep] = useState(0);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState('');
  const detect = async () => {
    setBusy(true); setError('');
    try { const s = await getDSHStatus(); setStatus(s); setStep(s.found ? 1 : 0); }
    catch (e) { setError(e instanceof Error ? e.message : String(e)); }
    finally { setBusy(false); }
  };
  useEffect(() => { void detect(); }, []);
  const install = async () => {
    setBusy(true); setError('');
    try { const s = await installDSHPlugin(lang); setStatus(s); setStep(2); }
    catch (e) { setError(e instanceof Error ? e.message : String(e)); }
    finally { setBusy(false); }
  };
  return <div className="space-y-4 max-w-3xl" data-testid="dsh-setup">
    <h2 className="text-lg font-bold">{t.title}</h2><p className="text-sm text-gray-500">{t.note}</p>
    <div className="flex flex-wrap gap-2">{t.steps.map((label, i) => <span key={label}
      className={`rounded-full px-3 py-1 text-sm ${step === i ? 'bg-accent text-white' : 'bg-gray-100 dark:bg-gray-800'}`}>{i + 1}. {label}</span>)}</div>
    <Card className="space-y-4">
      {busy && <p>{t.busy}</p>}
      {error && <p role="alert" className="text-red-500">{error}</p>}
      {step === 0 && <><p>{status && !status.found ? t.missing : t.steps[0]}</p><Button onClick={detect} disabled={busy}>{t.detect}</Button></>}
      {step === 1 && <><p>DSH {status?.version}</p><Button onClick={install} disabled={busy} data-testid="dsh-install-plugin">{t.install}</Button></>}
      {step === 2 && <><p>{t.open}</p><pre className="overflow-auto p-3 bg-gray-100 dark:bg-gray-800 rounded" data-testid="dsh-open-command">{status?.open_command}</pre></>}
      <Button variant="secondary" onClick={onBack} disabled={busy}>{t.back}</Button>
    </Card>
  </div>;
}
