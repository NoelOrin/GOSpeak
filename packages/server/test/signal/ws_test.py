#!/usr/bin/env python3
"""
Socket.IO WebSocket 集成测试
直接通过 WebSocket 连接测试 Socket.IO 事件
"""
import asyncio
import json
import websockets

BASE_HOST = "localhost"
BASE_PORT = 8998

def log(desc, ok, detail=""):
    icon = "✅" if ok else "❌"
    d = f": {detail}" if detail else ""
    print(f"  {icon} {desc}{d}")

def separator(title):
    print(f"\n{'='*56}")
    print(f"  {title}")
    print(f"{'='*56}")

# ─── Engine.IO/Socket.IO 协议常量 ───
# EIO: 0=open, 4=message
# SIO: 0=CONNECT, 2=EVENT

async def ws_connect():
    """直接 WebSocket 连接（Engine.IO 会自动完成握手）"""
    uri = f"ws://{BASE_HOST}:{BASE_PORT}/socket.io/?EIO=4&transport=websocket"
    ws = await websockets.connect(
        uri,
        additional_headers={"Origin": f"http://{BASE_HOST}:{BASE_PORT}"},
        open_timeout=5,
    )
    return ws

async def recv_eio(ws, timeout=5):
    """接收一帧 Engine.IO 数据（返回原始字符串）"""
    try:
        raw = await asyncio.wait_for(ws.recv(), timeout=timeout)
        if isinstance(raw, bytes):
            raw = raw.decode("utf-8", errors="replace")
        return raw
    except asyncio.TimeoutError:
        return None

async def send_sio_event(ws, event, data):
    """发送 Socket.IO EVENT 包: 42["event",{...}]"""
    payload = json.dumps(data)
    pkt = f'42["{event}",{payload}]'
    await ws.send(pkt)

def parse_sio_event(msg):
    """解析 Socket.IO 消息，返回 (packet_type, event_name, data)"""
    if not msg or len(msg) < 2:
        return None, None, None
    # Engine.IO type prefix (already stripped for message type 4)
    sio_type = msg[0]
    if sio_type == "0":
        return "CONNECT", None, msg[1:]
    if sio_type == "4" and len(msg) > 1:
        # Nested: "40" = CONNECT, "42" = EVENT
        inner = msg[1]
        if inner == "0":
            return "CONNECT", None, msg[2:]
        if inner == "2":
            rest = msg[2:]
            try:
                arr = json.loads(rest)
                if isinstance(arr, list) and len(arr) >= 1:
                    return "EVENT", arr[0], arr[1] if len(arr) > 1 else None
            except json.JSONDecodeError:
                pass
            return "EVENT", None, rest
    if sio_type == "2":
        rest = msg[1:]
        try:
            arr = json.loads(rest)
            if isinstance(arr, list) and len(arr) >= 1:
                return "EVENT", arr[0], arr[1] if len(arr) > 1 else None
        except json.JSONDecodeError:
            pass
        return "EVENT", None, rest
    return "OTHER", None, msg

async def drain_messages(ws, max_count=10, timeout=1):
    """排空接收队列，收集所有待接收消息"""
    msgs = []
    for _ in range(max_count):
        try:
            raw = await asyncio.wait_for(ws.recv(), timeout=timeout)
            if isinstance(raw, bytes):
                raw = raw.decode("utf-8", errors="replace")
            msgs.append(raw)
        except asyncio.TimeoutError:
            break
    return msgs

async def wait_for_event(ws, event_name, timeout=5):
    """等待特定事件"""
    deadline = asyncio.get_event_loop().time() + timeout
    while True:
        remaining = deadline - asyncio.get_event_loop().time()
        if remaining <= 0:
            return None
        raw = await recv_eio(ws, min(remaining, 2))
        if raw is None:
            return None
        ptype, ename, edata = parse_sio_event(raw)
        if ptype == "EVENT" and ename == event_name:
            return edata
        # 继续等待

# ─── 测试用例 ───

async def test_connection():
    separator("Socket.IO 连接测试")

    ws = await ws_connect()
    log("WebSocket 连接建立", True)

    # Engine.IO 会首先发送 OPEN 包（EIO type 0）
    raw = await recv_eio(ws, timeout=5)
    is_open = raw and raw.startswith("0{")
    log("收到 Engine.IO OPEN 包", is_open, raw[:60] if raw else "超时")

    # 客户端必须主动发送 Socket.IO CONNECT 包: 0
    await ws.send("40")
    log("发送 Socket.IO CONNECT", True)

    # 等待服务器响应 Socket.IO CONNECT
    raw2 = await recv_eio(ws, timeout=5)
    ptype, _, _ = parse_sio_event(raw2 or "")
    log("收到 Socket.IO CONNECT 响应", ptype == "CONNECT", raw2[:60] if raw2 else "超时")

    return ws

async def test_room_create(ws):
    separator("Socket.IO room:create 测试")

    # 正常创建房间
    await send_sio_event(ws, "room:create", {"room": "ws-test-room", "identity": "tester-1"})
    data = await wait_for_event(ws, "room:created", timeout=5)
    log("收到 room:created 事件", data is not None,
        f"room={data.get('name','?')}, members={data.get('count','?')}" if data else "无响应")

    # 空房间名
    await send_sio_event(ws, "room:create", {"room": "", "identity": "tester"})
    raw = await recv_eio(ws, timeout=3)
    _, ename, edata = parse_sio_event(raw or "")
    log("空房间名返回错误", edata is not None and "error" in str(edata).lower(), str(edata)[:60] if edata else "无响应")

    # 重复房间名
    await send_sio_event(ws, "room:create", {"room": "ws-test-room", "identity": "tester-2"})
    raw2 = await recv_eio(ws, timeout=3)
    _, ename2, edata2 = parse_sio_event(raw2 or "")
    log("重复创建返回错误", edata2 is not None and "error" in str(edata2).lower(), str(edata2)[:60] if edata2 else "无响应")

async def test_room_join(ws):
    separator("Socket.IO room:join 测试")

    await send_sio_event(ws, "room:join", {"room": "ws-test-room", "identity": "player-1"})
    data = await wait_for_event(ws, "room:joined", timeout=5)
    log("收到 room:joined 事件", data is not None,
        f"room={data.get('room','?')}, count={data.get('count','?')}" if data else "无响应")

    has_members = data is not None and "members" in data
    log("包含 members 列表", has_members)

async def test_room_list(ws):
    separator("Socket.IO room:list 测试")

    await send_sio_event(ws, "room:list", {})
    data = await wait_for_event(ws, "room:list:result", timeout=5)
    log("收到 room:list:result", data is not None,
        f"rooms={len(data.get('rooms',[]))}, count={data.get('count','?')}" if data else "无响应")

async def test_room_leave(ws):
    separator("Socket.IO room:leave 测试")

    await send_sio_event(ws, "room:leave", {"room": "ws-test-room"})
    data = await wait_for_event(ws, "room:left", timeout=5)
    log("收到 room:left 事件", data is not None,
        f"room={data.get('room','?')}" if data else "无响应")

async def test_multi_client():
    separator("Socket.IO 多客户端测试")

    # 客户端 A
    ws_a = await ws_connect()
    await recv_eio(ws_a, timeout=5)  # OPEN
    await ws_a.send("40")  # SIO CONNECT
    await recv_eio(ws_a, timeout=5)  # SIO CONNECT response
    log("客户端 A 连接", True)

    # A 创建房间
    await send_sio_event(ws_a, "room:create", {"room": "multi-room", "identity": "user-A"})
    await wait_for_event(ws_a, "room:created", timeout=5)
    log("A 创建房间 multi-room", True)

    # 客户端 B
    ws_b = await ws_connect()
    await recv_eio(ws_b, timeout=5)  # OPEN
    await ws_b.send("40")  # SIO CONNECT
    await recv_eio(ws_b, timeout=5)  # SIO CONNECT response
    log("客户端 B 连接", True)

    # B 加入房间
    await send_sio_event(ws_b, "room:join", {"room": "multi-room", "identity": "user-B"})
    data_b = await wait_for_event(ws_b, "room:joined", timeout=5)
    log("B 收到 room:joined", data_b is not None,
        f"count={data_b.get('count','?')}" if data_b else "无响应")

    # A 应收到 member:joined 通知
    data_a = await wait_for_event(ws_a, "member:joined", timeout=5)
    log("A 收到 member:joined 通知", data_a is not None,
        f"identity={data_a.get('identity','?')}" if data_a else "无响应")

    # B 离开房间
    await send_sio_event(ws_b, "room:leave", {"room": "multi-room"})
    await wait_for_event(ws_b, "room:left", timeout=3)

    # A 应收到 member:left 通知
    data_left = await wait_for_event(ws_a, "member:left", timeout=5)
    log("A 收到 member:left 通知", data_left is not None,
        f"identity={data_left.get('identity','?')}" if data_left else "无响应")

    # A 清理
    await send_sio_event(ws_a, "room:leave", {"room": "multi-room"})
    await wait_for_event(ws_a, "room:left", timeout=3)

    await ws_a.close()
    await ws_b.close()
    log("多客户端测试完成", True)

# ─── 入口 ───

async def main():
    print("\n🔌 Socket.IO WebSocket 集成测试\n")

    ws = None
    try:
        ws = await test_connection()
        if ws:
            await test_room_create(ws)
            await test_room_join(ws)
            await test_room_list(ws)
            await test_room_leave(ws)
            await ws.close()
    except Exception as e:
        log("单客户端测试异常", False, str(e))
        if ws:
            try:
                await ws.close()
            except:
                pass

    try:
        await test_multi_client()
    except Exception as e:
        log("多客户端测试异常", False, str(e))

    print("\n" + "-" * 56)

if __name__ == "__main__":
    asyncio.run(main())
