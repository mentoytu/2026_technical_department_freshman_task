// 后端实时监视器：做第 4 步（前端操作 → 后端数据）验证时用
// 用法： node watch.mjs          默认 http://127.0.0.1:8080
//        node watch.mjs 8081     指定端口
// 作用：每隔 1.5 秒拉一次 GET /api/items，把「新增 / 修改 / 删除」实时打印在终端。
//       你在浏览器里操作前端，这里就会跟着出现 + / ~ / - 行。

const PORT = process.argv[2] || '8080';
const BASE = 'http://127.0.0.1:' + PORT + '/api';
const INTERVAL = 1500;

let prev = new Map();
let firstRun = true;
let errorPrinted = false;

const stamp = () => new Date().toLocaleTimeString('zh-CN', { hour12: false });

function line(it) {
  const imgs = Array.isArray(it.imgs) ? it.imgs.length : 0;
  return [
    '#' + it.id,
    it.title,
    it.type === 'found' ? '招领' : '失物',
    it.status === 'done' ? (it.type === 'found' ? '已归还' : '已找到') : (it.type === 'found' ? '认领中' : '寻找中'),
    '发布人=' + (it.owner || ''),
    '地点=' + (it.place || '-'),
    '电话=' + (it.phone || '-'),
    '图片=' + imgs + '张'
  ].join('  |  ');
}

function changedFields(a, b) {
  const keys = ['title', 'desc', 'type', 'status', 'owner', 'campus', 'place', 'place_info', 'phone', 'date'];
  const out = [];
  keys.forEach((k) => { if ((a[k] || '') !== (b[k] || '')) out.push(k + ': ' + (a[k] || '空') + ' → ' + (b[k] || '空')); });
  const ai = (a.imgs || []).length, bi = (b.imgs || []).length;
  if (ai !== bi) out.push('imgs: ' + ai + '张 → ' + bi + '张');
  return out.join(' ; ') || '内容有变化';
}

async function tick() {
  try {
    const res = await fetch(BASE + '/items');
    const data = await res.json();
    const list = data.items || [];
    const cur = new Map(list.map((i) => [i.id, i]));

    if (firstRun) {
      console.log('\n已连接后端 ' + BASE + '（Ctrl+C 退出）');
      console.log('当前后端数据 ' + list.length + ' 条：');
      list.forEach((it) => console.log('   ' + line(it)));
      console.log('\n现在去浏览器操作前端，这里会实时打印变化：\n');
      prev = cur;
      firstRun = false;
      errorPrinted = false;
      return;
    }

    const events = [];
    for (const [id, it] of cur) {
      if (!prev.has(id)) events.push('+ ' + stamp() + ' 新增   ' + line(it));
      else if (JSON.stringify(prev.get(id)) !== JSON.stringify(it)) {
        events.push('~ ' + stamp() + ' 修改   #' + id + ' ' + it.title + '\n            ' + changedFields(prev.get(id), it));
      }
    }
    for (const [id, it] of prev) if (!cur.has(id)) events.push('- ' + stamp() + ' 删除   #' + id + ' ' + it.title);

    if (events.length) {
      events.forEach((e) => console.log(e));
      console.log('  → 后端现有 ' + list.length + ' 条：' + list.map((i) => '#' + i.id + ' ' + i.title).join('、') + '\n');
    }
    prev = cur;
    errorPrinted = false;
  } catch (e) {
    if (!errorPrinted) {
      console.log('[' + stamp() + '] 连不上后端（' + e.message + '），请确认 go run main.go 还在运行、端口是 ' + PORT);
      errorPrinted = true;
    }
  }
}

console.log('正在连接 ' + BASE + ' ...');
tick();
setInterval(tick, INTERVAL);
