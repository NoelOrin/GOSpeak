// 基线比对。用法：node compare.mjs <routeId> <actual.png> [theme=light]
// 判定：ratio < 0.02 通过；0.02–0.05 交视觉审查判定；> 0.05 不通过。
import { PNG } from "pngjs";
import pixelmatch from "pixelmatch";
import { readFile } from "node:fs/promises";
import path from "node:path";
import { fileURLToPath } from "node:url";

const here = path.dirname(fileURLToPath(import.meta.url));
const [routeId, actualPath, theme = "light"] = process.argv.slice(2);
if (!routeId || !actualPath) {
  console.error("usage: node compare.mjs <routeId> <actual.png> [theme]");
  process.exit(2);
}

const [a, b] = await Promise.all([
  readFile(path.join(here, "baseline", `${routeId}-${theme}.png`)),
  readFile(actualPath),
]);
const pa = PNG.sync.read(a);
const pb = PNG.sync.read(b);
const width = Math.min(pa.width, pb.width);
const height = Math.min(pa.height, pb.height);

function crop(png) {
  const out = Buffer.alloc(width * height * 4);
  for (let y = 0; y < height; y++) {
    png.data.copy(out, y * width * 4, y * png.width * 4, y * png.width * 4 + width * 4);
  }
  return out;
}

const diff = new PNG({ width, height });
const mismatched = pixelmatch(crop(pa), crop(pb), diff.data, width, height, { threshold: 0.1 });
const ratio = +(mismatched / (width * height)).toFixed(4);
const verdict = ratio < 0.02 ? "pass" : ratio <= 0.05 ? "review" : "fail";
console.log(JSON.stringify({ routeId, theme, mismatched, total: width * height, ratio, verdict }));
