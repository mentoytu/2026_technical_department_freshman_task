// 前后端契约验证：按前端 campus-lost-found.html 发出的完全相同的请求体调用后端
// 用法： node api-test.mjs            默认 http://127.0.0.1:8080
//        node api-test.mjs 8081       指定端口
const PORT = process.argv[2] || '8080';
const BASE = 'http://127.0.0.1:' + PORT + '/api';
const log = (...a) => console.log(...a);
let token = '';

async function req(path, opts = {}) {
  const headers = { 'Content-Type': 'application/json' };
  if (token) headers.Authorization = 'Bearer ' + token;
  const res = await fetch(BASE + path, {
    method: opts.method || 'GET',
    headers,
    body: opts.body ? JSON.stringify(opts.body) : undefined
  });
  const data = await res.json().catch(() => null);
  return { status: res.status, data };
}

(async () => {
  let ok = 0, bad = 0;
  const check = (label, cond, extra) => {
    if (cond) { ok++; log('  [PASS] ' + label, extra === undefined ? '' : extra); }
    else { bad++; log('  [FAIL] ' + label, extra === undefined ? '' : extra); }
  };

  log('\n== 1. 列表 ==');
  let r = await req('/items');
  check('GET /api/items 返回 200', r.status === 200, 'total=' + r.data.total);

  log('\n== 2. 登录（前端：先 login，401 则自动 register 再 login） ==');
  const cred = { student_id: '20230012345', password: '123456' };
  r = await req('/login', { method: 'POST', body: cred });
  log('  首次 login ->', r.status, JSON.stringify(r.data));
  if (r.status !== 200) {
    const g = await req('/register', { method: 'POST', body: Object.assign({ nickname: '测试同学' }, cred) });
    log('  自动 register ->', g.status, JSON.stringify(g.data));
    r = await req('/login', { method: 'POST', body: cred });
  }
  check('login 返回 token', r.status === 200 && !!r.data.token, r.data.token);
  token = r.data.token;

  log('\n== 3. 发布（前端字段形状，含图片/地点/联系方式） ==');
  const payload = {
    title: '银色保温杯', desc: '杯身有蓝色校徽贴纸', type: 'lost', status: 'searching',
    owner: '测试同学', campus: '东校区', place: '实验楼', place_info: '三层走廊 · 靠近饮水机',
    phone: '13800001234', date: new Date().toISOString().slice(0, 10),
    imgs: [{ thumb: 'data:image/jpeg;base64,AAAA', large: 'data:image/jpeg;base64,BBBB' }]
  };
  r = await req('/items', { method: 'POST', body: payload });
  check('POST /api/items 成功', r.status === 200 && r.data.item.id > 0, 'id=' + r.data.item.id);
  check('owner 自动取 Token 对应的人? (body 优先)', r.data.item.owner === '测试同学', r.data.item.owner);
  const id = r.data.item.id;

  log('\n== 4. 回读：确认前端要用的字段没丢 ==');
  r = await req('/items');
  const got = r.data.items.find((x) => x.id === id);
  check('place 保留', got.place === '实验楼', got.place);
  check('place_info 保留', got.place_info === '三层走廊 · 靠近饮水机', got.place_info);
  check('phone 保留', got.phone === '13800001234', got.phone);
  check('campus 保留', got.campus === '东校区', got.campus);
  check('imgs 保留 1 张', Array.isArray(got.imgs) && got.imgs.length === 1, JSON.stringify((got.imgs || []).length));

  log('\n== 5. 修改状态 ==');
  r = await req('/items/' + id, { method: 'PUT', body: { status: 'done' } });
  check('PUT 改状态成功', r.status === 200 && r.data.item.status === 'done', r.data.item.status);

  log('\n== 6. 关键词 + 状态筛选 ==');
  r = await req('/items?keyword=' + encodeURIComponent('保温杯') + '&status=done');
  check('关键词命中', r.status === 200 && r.data.total === 1, 'total=' + r.data.total);
  r = await req('/items?status=searching');
  check('状态筛选不含已 done 的', r.data.items.every((x) => x.status === 'searching'), 'total=' + r.data.total);

  log('\n== 7. 修改资料时同步 owner（前端改昵称会批量改 owner） ==');
  r = await req('/items/' + id, { method: 'PUT', body: { owner: '测试同学改名了', phone: '13900001111' } });
  check('PUT 改 owner 成功', r.status === 200 && r.data.item.owner === '测试同学改名了', r.data.item.owner);
  r = await req('/items');
  const got2 = r.data.items.find((x) => x.id === id);
  check('回读 owner 已更新', got2.owner === '测试同学改名了', got2.owner);

  log('\n== 8. 删除 ==');
  r = await req('/items/' + id, { method: 'DELETE' });
  check('DELETE 成功', r.status === 200, JSON.stringify(r.data));
  r = await req('/items');
  check('删除后查不到', !r.data.items.some((x) => x.id === id), 'total=' + r.data.total);

  log('\n== 9. 错误分支 ==');
  r = await req('/items/9999', { method: 'PUT', body: { status: 'done' } });
  check('改不存在 -> 404', r.status === 404, JSON.stringify(r.data));
  r = await req('/items', { method: 'POST', body: { title: '' } });
  check('空标题 -> 400', r.status === 400, JSON.stringify(r.data));
  r = await req('/items/abc', { method: 'DELETE' });
  check('非法 id -> 400', r.status === 400, JSON.stringify(r.data));

  log('\n==================== 结果：' + ok + ' 通过 / ' + bad + ' 失败 ====================');
  process.exit(bad ? 1 : 0);
})();
