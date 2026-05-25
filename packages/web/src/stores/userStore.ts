import { get, set, del, clear, keys } from "idb-keyval";

// 设置
await set("user", { name: "Alice", age: 30 });

// 获取
const user = await get("user"); // { name: 'Alice', age: 30 }

// 删除
await del("user");

// 清空
await clear();

// 获取所有 key
const allKeys = await keys();