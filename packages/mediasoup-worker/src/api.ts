import { Router as ExpressRouter } from "express";
import MediasoupWorker from "./worker";

export function createRouter(worker: MediasoupWorker): ExpressRouter {
	const router = ExpressRouter();

	router.get("/health", async (_req, res) => {
		const stats = await worker.getStats();
		res.json({ ok: true, ...stats });
	});

	router.get("/rooms", (_req, res) => {
		res.json({ rooms: worker.listRooms() });
	});

	router.post("/rooms", async (req, res) => {
		const { roomId } = req.body;
		if (!roomId) return res.status(400).json({ error: "roomId required" });
		try {
			const routerObj = await worker.createRouter(roomId);
			res.json({ roomId, rtpCapabilities: routerObj.rtpCapabilities });
		} catch (err) {
			res.status(500).json({ error: (err as Error).message });
		}
	});

	router.get("/rooms/:roomId/rtp-capabilities", (req, res) => {
		const routerObj = worker.getRouter(req.params.roomId);
		if (!routerObj) return res.status(404).json({ error: "room not found" });
		res.json({ rtpCapabilities: routerObj.rtpCapabilities });
	});

	router.delete("/rooms/:roomId", async (req, res) => {
		await worker.closeRouter(req.params.roomId);
		res.json({ ok: true });
	});

	router.post("/rooms/:roomId/transports", async (req, res) => {
		try {
			const transport = await worker.createTransport(req.params.roomId);
			res.json({
				id: transport.id,
				iceParameters: transport.iceParameters,
				iceCandidates: transport.iceCandidates,
				dtlsParameters: transport.dtlsParameters,
				sctpParameters: transport.sctpParameters,
			});
		} catch (err) {
			const message = (err as Error).message;
			res.status(message === "room not found" ? 404 : 500).json({ error: message });
		}
	});

	router.post("/rooms/:roomId/transports/:transportId/connect", async (req, res) => {
		const transport = worker.getTransport(req.params.roomId, req.params.transportId);
		if (!transport) return res.status(404).json({ error: "transport not found" });
		try {
			await transport.connect({ dtlsParameters: req.body.dtlsParameters });
			res.json({ ok: true });
		} catch (err) {
			res.status(500).json({ error: (err as Error).message });
		}
	});

	router.post("/rooms/:roomId/produce", async (req, res) => {
		const { transportId, kind, rtpParameters, appData } = req.body;
		const transport = worker.getTransport(req.params.roomId, transportId);
		if (!transport) return res.status(404).json({ error: "transport not found" });
		try {
			const producer = await transport.produce({ kind, rtpParameters, appData });
			worker.addProducer(req.params.roomId, producer);
			res.json({ id: producer.id, kind: producer.kind });
		} catch (err) {
			res.status(500).json({ error: (err as Error).message });
		}
	});

	router.post("/rooms/:roomId/consume", async (req, res) => {
		const { transportId, producerId, rtpCapabilities } = req.body;
		const room = worker.getRoom(req.params.roomId);
		if (!room) return res.status(404).json({ error: "room not found" });
		const transport = worker.getTransport(req.params.roomId, transportId);
		if (!transport) return res.status(404).json({ error: "transport not found" });
		try {
			if (!room.router.canConsume({ producerId, rtpCapabilities })) {
				return res.status(400).json({ error: "cannot consume" });
			}
			const consumer = await transport.consume({ producerId, rtpCapabilities, paused: false });
			worker.addConsumer(req.params.roomId, consumer);
			res.json({
				id: consumer.id,
				producerId: consumer.producerId,
				kind: consumer.kind,
				rtpParameters: consumer.rtpParameters,
			});
		} catch (err) {
			res.status(500).json({ error: (err as Error).message });
		}
	});

	router.get("/rooms/:roomId/producers", (req, res) => {
		if (!worker.getRoom(req.params.roomId)) return res.status(404).json({ error: "room not found" });
		res.json({ producers: worker.listProducers(req.params.roomId) });
	});

	return router;
}
