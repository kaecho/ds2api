#!/usr/bin/env node
/**
 * 数美 deviceId 纯协议生成（逆向自 cdn.deepseek.com/static/chat/fp-1.min.js）
 * 输出: B{deviceId}  或错误到 stderr
 */
'use strict';
const crypto = require('crypto');
const zlib = require('zlib');
const https = require('https');
const fs = require('fs');
const path = require('path');
const { DES } = require('./sm_des.js');

const CONF = JSON.parse(fs.readFileSync(path.join(__dirname, 'sm_conf.json'), 'utf8'));
const ORG = 'P9usCUBauxft8eAmUXaZ';
const APP = 'default';
const API_HOST = process.env.SM_API_HOST || 'fp-it-acc.portal101.cn';
const PUB_PEM = `-----BEGIN PUBLIC KEY-----
MIGfMA0GCSqGSIb3DQEBAQUAA4GNADCBiQKBgQDetfEgYD4aE1ZjmWJ6/jnPurhzI+ye
RoJHWrnNtQMte3stQ4VjG3yu21FuN75E6cDpA9KtDXwcB2M/FiGUAe3G0rNotbWI8+Sj
ZfUbW/OILFTzY0uaeEkmVGW5WyJ6weQbbr1xTCPa2OO3YIMeZljWUYHG5h21WAm/PATg
8im8cQIDAQAB
-----END PUBLIC KEY-----`;

const UA = process.env.SM_UA || (
  'Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) ' +
  'Chrome/131.0.0.0 Safari/537.36'
);
const PLATFORM = process.env.SM_PLATFORM || 'MacIntel';

// 常见桌面分辨率：res=屏宽_屏高_可用宽_可用高；clientSize 跟窗口自洽
const SCREEN_PROFILES = [
  { res: '1920_1080_1920_1040', clientSize: '1920_937_1920_1040' },
  { res: '1920_1080_1920_1040', clientSize: '1536_791_1536_864' },
  { res: '2560_1440_2560_1400', clientSize: '2560_1295_2560_1400' },
  { res: '2560_1440_2560_1400', clientSize: '1920_1009_1920_1080' },
  { res: '1440_900_1440_860', clientSize: '1440_757_1440_860' },
  { res: '1536_864_1536_824', clientSize: '1536_721_1536_824' },
  { res: '1680_1050_1680_1010', clientSize: '1680_907_1680_1010' },
  { res: '1366_768_1366_728', clientSize: '1366_625_1366_728' },
  { res: '2880_1800_2880_1740', clientSize: '1440_821_1440_900' }, // retina 逻辑像素
  { res: '3024_1964_3024_1904', clientSize: '1512_916_1512_982' },
];
const CPU_COUNTS = [4, 6, 8, 10, 12, 16];

function pick(arr) {
  return arr[(Math.random() * arr.length) | 0];
}
function randInt(min, max) {
  return min + ((Math.random() * (max - min + 1)) | 0);
}
function randFloat(min, max, digits) {
  const v = min + Math.random() * (max - min);
  return Number(v.toFixed(digits == null ? 2 : digits));
}

function uuid() {
  return 'xxxxxxxx-xxxx-4xxx-yxxx-xxxxxxxxxxxx'.replace(/[xy]/g, c => {
    const r = (Math.random() * 16) | 0;
    return (c === 'x' ? r : (r & 3) | 8).toString(16);
  });
}
function md5(s) {
  return crypto.createHash('md5').update(String(s)).digest('hex');
}
function btoaBin(s) {
  return Buffer.from(String(s), 'binary').toString('base64');
}
function pad2(n) {
  n = String(n);
  return n.length < 2 ? '0' + n : n;
}

/** 本地 smidV2：yyyymmddHHMMSS + md5(uuid) + '00' + md5('smsk_web_'+..).substr(0,14) + '0' */
function getLocalsmid() {
  const d = new Date();
  const ts =
    d.getFullYear().toString() +
    pad2(d.getMonth() + 1) +
    pad2(d.getDate()) +
    pad2(d.getHours()) +
    pad2(d.getMinutes()) +
    pad2(d.getSeconds());
  const u = uuid();
  const mid = ts + md5(u) + '00';
  const tail = md5('smsk_web_' + mid).substr(0, 14);
  return mid + tail + '0';
}

/** tn：对象 key 排序后递归拼接；number 先 *10000 再转字符串；最后 md5 */
function tnOf(obj) {
  function walk(v) {
    if (Object.prototype.toString.call(v) === '[object Object]') {
      const parts = [];
      Object.keys(v)
        .sort()
        .forEach(k => {
          if (typeof v[k] === 'number') parts.push(walk(String(10000 * v[k])));
          else parts.push(walk(String(v[k])));
        });
      return parts.join('');
    }
    return v ? String(v) : '';
  }
  return md5(walk(obj));
}

function desField(key, value) {
  return btoaBin(DES(key, String(value), 1, 0));
}

function confuse(box, confData) {
  const out = {};
  for (const k of Object.keys(box)) {
    const rule = confData[k];
    if (!rule) {
      out[k] = box[k];
      continue;
    }
    let val = box[k];
    if (rule.is_encrypt && val != null && val !== '') {
      if (rule.cipher === 'DES') val = desField(rule.key, val);
    }
    out[rule.obfuscated_name] = val;
  }
  return out;
}

function zeroPad(buf) {
  const bs = 16;
  const n = bs - (buf.length % bs || bs);
  return Buffer.concat([buf, Buffer.alloc(n, 0)]);
}

function aesEncryptHex(plaintext, priId) {
  // gzip() in SDK: JSON.stringify -> pako.gzip to binary string -> btoa
  // then AES-CBC ZeroPadding key=priId, iv=0102030405060708, ciphertext.toString() => hex
  const gz = zlib.gzipSync(Buffer.from(plaintext, 'utf8'));
  const gzB64 = gz.toString('base64');
  const iv = Buffer.from('0102030405060708');
  const key = Buffer.from(priId, 'utf8');
  const cipher = crypto.createCipheriv('aes-128-cbc', key, iv);
  cipher.setAutoPadding(false);
  const enc = Buffer.concat([cipher.update(zeroPad(Buffer.from(gzB64, 'utf8'))), cipher.final()]);
  return enc.toString('hex');
}

function buildBox(prevBox) {
  const now = Date.now();
  const uid = uuid();
  const screen = pick(SCREEN_PROFILES);
  // canvas：真实 SDK 是 hash(toDataURL)；这里用随机盐避免同 UA 同 canvas
  const canvas = md5(UA + '|' + uid + '|' + crypto.randomBytes(16).toString('hex')).slice(0, 32);
  const box = {
    protocol: CONF.Protocol, // 0x85 = 133
    organization: ORG,
    appId: APP,
    os: 'web',
    version: '3.0.0',
    sdkver: '3.0.0',
    box: prevBox || '',
    rtype: 'all',
    smid: getLocalsmid(),
    subVersion: '1.0.0',
    time: now / 1000,
    plugins:
      'Chrome PDF Viewer::application/pdf~pdf,Chromium PDF Viewer::application/pdf~pdf,Microsoft Edge PDF Viewer::application/pdf~pdf,PDF Viewer::application/pdf~pdf,WebKit built-in PDF::application/pdf~pdf',
    ua: UA,
    canvas,
    timezone: -new Date().getTimezoneOffset(),
    platform: PLATFORM,
    url: 'https://platform.deepseek.com/sign_up',
    referer: '',
    res: screen.res,
    clientSize: screen.clientSize,
    status: 'true',
    vpw: uid,
    svm: now,
    trees: uuid(),
    pmf: now,
    cdp: 0,
    maxTouchPoints: 0,
    connectionRtt: randInt(20, 120),
    cpucount: pick(CPU_COUNTS),
    battery: {
      charging: Math.random() < 0.55 ? 1 : 0,
      level: randFloat(0.28, 1.0, 2),
    },
  };
  // tn 两次：先整体 md5 写入，confuse 前再算一次（与 SDK 一致）
  box.tn = tnOf(box);
  box.tn = tnOf(box);
  return { box, uid };
}

function postJson(host, apiPath, body) {
  const data = JSON.stringify(body);
  return new Promise((resolve, reject) => {
    const req = https.request(
      {
        host,
        path: apiPath,
        method: 'POST',
        headers: {
          'Content-Type': 'application/json;charset=UTF-8',
          Origin: 'https://platform.deepseek.com',
          Referer: 'https://platform.deepseek.com/',
          'User-Agent': UA,
          'Content-Length': Buffer.byteLength(data),
        },
        timeout: 20000,
      },
      res => {
        let buf = '';
        res.on('data', c => (buf += c));
        res.on('end', () => resolve({ status: res.statusCode, body: buf }));
      }
    );
    req.on('error', reject);
    req.on('timeout', () => req.destroy(new Error('timeout')));
    req.write(data);
    req.end();
  });
}

async function main() {
  const prev = process.env.SM_PREV_BOX || '';
  const { box, uid } = buildBox(prev);
  const priId = md5(uid).slice(0, 16);
  const ep = crypto
    .publicEncrypt({ key: PUB_PEM, padding: crypto.constants.RSA_PKCS1_PADDING }, Buffer.from(uid))
    .toString('base64');

  const confused = confuse(box, CONF.ConfusionInfo.data);
  // SDK: gzip(confused object) — JSON.stringify inside gzip helper
  const dataHex = aesEncryptHex(JSON.stringify(confused), priId);

  const payload = {
    appId: APP,
    organization: ORG,
    ep,
    data: dataHex,
    os: 'web',
    encode: 5,
    compress: 2,
  };

  const resp = await postJson(API_HOST, '/deviceprofile/v4', payload);
  let parsed;
  try {
    parsed = JSON.parse(resp.body);
  } catch (e) {
    console.error('bad response', resp.status, resp.body.slice(0, 200));
    process.exit(2);
  }
  if (Number(parsed.code) === 1100 && parsed.detail && parsed.detail.deviceId) {
    const did = 'B' + parsed.detail.deviceId;
    process.stdout.write(did);
    return;
  }
  console.error(JSON.stringify(parsed));
  process.exit(1);
}

main().catch(e => {
  console.error(e && e.stack ? e.stack : e);
  process.exit(1);
});
