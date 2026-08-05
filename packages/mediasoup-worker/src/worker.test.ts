import assert from "node:assert/strict";
import test from "node:test";
import MediasoupWorker from "./worker";

function fakeObserver() {
	return { on: () => {} };
}

function makeTransport(id: string) {
	const transport: any = {
		id,
		closed: false,
		observer: fakeObserver(),
		close() {
			transport.closed = true;
		},
	};
	return transport;
}

function makeProducer(id: string, identity: string) {
	const producer: any = {
		id,
		kind: "audio",
		appData: { identity },
		closed: false,
		observer: fakeObserver(),
		close() {
			producer.closed = true;
		},
		pause() {},
		resume() {},
	};
	return producer;
}

function makeConsumer(id: string) {
	const consumer: any = {
		id,
		closed: false,
		observer: fakeObserver(),
		close() {
			consumer.closed = true;
		},
	};
	return consumer;
}

function makeRouter() {
	const router: any = {
		closed: false,
		observer: fakeObserver(),
		createWebRtcTransport: async () => makeTransport("transport-1"),
		close() {
			router.closed = true;
		},
	};
	return router;
}

function makeWorker(router: any) {
	const worker: any = {
		pid: 123,
		closed: false,
		observer: fakeObserver(),
		createRouter: async () => router,
		close() {
			worker.closed = true;
		},
	};
	return worker;
}

test("closeRouter explicitly closes room resources", async () => {
	const router = makeRouter();
	const worker = new MediasoupWorker({ createWorker: async () => makeWorker(router) });
	await worker.init();
	await worker.createRouter("room-1");

	const send = await worker.createTransport("room-1", "alice", "send");
	const producer = makeProducer("p-1", "alice");
	worker.addProducer("room-1", producer);
	const consumer = makeConsumer("c-1");
	worker.addConsumer("room-1", consumer);

	const room = worker.getRoom("room-1");
	assert.equal(room?.status, "open");
	assert.equal(worker.getParticipant("room-1", "alice")?.status, "joined");

	await worker.closeRouter("room-1");

	assert.deepEqual(worker.listRooms(), []);
	assert.equal(room?.status, "closed");
	assert.equal(router.closed, true);
	assert.equal(send.closed, true);
	assert.equal(producer.closed, true);
	assert.equal(consumer.closed, true);
});

test("closeParticipant explicitly marks and cleans participant", async () => {
	const worker = new MediasoupWorker({ createWorker: async () => makeWorker(makeRouter()) });
	await worker.init();
	await worker.createRouter("room-1");

	const send = await worker.createTransport("room-1", "alice", "send");
	const producer = makeProducer("p-1", "alice");
	worker.addProducer("room-1", producer);

	const closedIds = worker.closeParticipant("room-1", "alice");
	assert.deepEqual(closedIds, ["p-1"]);
	assert.equal(producer.closed, true);
	assert.equal(send.closed, true);
	assert.equal(worker.getParticipant("room-1", "alice"), undefined);
	assert.deepEqual(worker.listParticipants("room-1"), []);
});

test("close explicitly closes all rooms and worker", async () => {
	const router = makeRouter();
	const worker = new MediasoupWorker({ createWorker: async () => makeWorker(router) });
	await worker.init();
	await worker.createRouter("room-1");
	await worker.createRouter("room-2");

	await worker.close();

	assert.deepEqual(worker.listRooms(), []);
	assert.equal(router.closed, true);
	const stats = await worker.getStats();
	assert.equal(stats.workerAlive, false);
});
