# Changelog / 更新日志

## [0.4.0](https://github.com/NoelOrin/GOSpeak/compare/v0.3.2...0.4.0) (2026-09-03)


### Features

* add cluster module, improve storage service, and polish frontend UI ([3318098](https://github.com/NoelOrin/GOSpeak/commit/33180987c327bfc89a9676e5114971f645c0f9a5))
* add domain role management APIs and enforce domain permissions ([a8ddb7b](https://github.com/NoelOrin/GOSpeak/commit/a8ddb7bbd15311dd69a7def5ec37106859c857c9))
* add domain role management UI ([f3bb36f](https://github.com/NoelOrin/GOSpeak/commit/f3bb36f64b8fb79c03d9e610b640b5f9a8bee35c))
* add domain role repository and seed ([ffa5e78](https://github.com/NoelOrin/GOSpeak/commit/ffa5e789c0e8b05c155d0b306a27f9da6637ea0f))
* add frontend domain role API and permission cache ([f91ab2b](https://github.com/NoelOrin/GOSpeak/commit/f91ab2bb4cdeba52dce85815b9d1138fa5fff711))
* add per-domain role and permission service ([c22d11e](https://github.com/NoelOrin/GOSpeak/commit/c22d11ed7be79f872c867dcdc5b733d64158f91e))
* add per-domain role models ([54fa72f](https://github.com/NoelOrin/GOSpeak/commit/54fa72f28d422383d5c89b5ff34fee049c693a87))
* **android:** add Android TWA scaffold ([8ecf97a](https://github.com/NoelOrin/GOSpeak/commit/8ecf97aa91e15b7ebf20a696401aaf68cb49584e))
* **audit:** add audit log module and API routes ([4c9580a](https://github.com/NoelOrin/GOSpeak/commit/4c9580a057c058a2eb75b0fe5296f425afacfd33))
* **auth:** access/refresh TTL 支持 JWT_ACCESS_TTL/JWT_REFRESH_TTL 配置 ([312a4dc](https://github.com/NoelOrin/GOSpeak/commit/312a4dc2954e4cdc17de0e771144c9f27d261254))
* **auth:** add auth middleware improvements and user service enhancements ([18b08cf](https://github.com/NoelOrin/GOSpeak/commit/18b08cf6eb0db2128ab9a2ee6dddba47d1b7e28b))
* **auth:** login/register/reset/refresh/oauth 响应下发 expires_in ([3c4991a](https://github.com/NoelOrin/GOSpeak/commit/3c4991ac9b732514b9f4c864317087c78065e92d))
* **auth:** 会话惰性续期前端链路与登录限流分桶 ([e3c0c74](https://github.com/NoelOrin/GOSpeak/commit/e3c0c7409a8afe0065041388164e1f75dc993e61))
* **auth:** 加固 JWT 鉴权与用户状态管理 ([2f74b07](https://github.com/NoelOrin/GOSpeak/commit/2f74b0714185ddc210215fa3f85099287e696c64))
* **backend:** update message service and signal hub ([322dd93](https://github.com/NoelOrin/GOSpeak/commit/322dd93632d1f395be57ad480f202e4e486d026d))
* **bot:** add media routing, TTS, ASR and WS ticket auth ([9640494](https://github.com/NoelOrin/GOSpeak/commit/96404948beb7a1454a2f72d789de70cb3546c196))
* **bot:** improve socket client capabilities and add tests ([7783288](https://github.com/NoelOrin/GOSpeak/commit/7783288471beff8631c2f44d79e8bd6a225acebf))
* **bus:** add ConcurrentDeliverer with concurrent fanout to all SIO clients ([4c593a2](https://github.com/NoelOrin/GOSpeak/commit/4c593a21fa522098a5e952c52ce128c3952adcce))
* **bus:** Redis 成员 CAS 与 JetStream 消息去重 ([eb2c0ef](https://github.com/NoelOrin/GOSpeak/commit/eb2c0ef1268493cb118fb17c04be298dee293d17))
* **bus:** 强化共享状态存储与断线恢复 ([b281ea9](https://github.com/NoelOrin/GOSpeak/commit/b281ea99eb53181aa4eb63193ba51cee793c32b1))
* **chat:** add direct message model and repository layer ([9179d5f](https://github.com/NoelOrin/GOSpeak/commit/9179d5fc06a5bd5df9593d3390a7855b9adcb032))
* **chat:** add DM service, handler, router, and DI wiring ([dd112cc](https://github.com/NoelOrin/GOSpeak/commit/dd112cc80cf82d261f87a36128c243b6cd235487))
* **chat:** add DM signal events and personal room routing ([13af4e0](https://github.com/NoelOrin/GOSpeak/commit/13af4e034ede5cf8af98fe28fa6a2387914a1f9e))
* **chat:** add frontend conversation API, chat store, and IDB cache ([aec942f](https://github.com/NoelOrin/GOSpeak/commit/aec942fd021cb31cdbe4b8c88134d8feb57fa7b2))
* **chat:** add private chat UI with conversation list, chat window, member sidebar, and /chat route ([97f0d62](https://github.com/NoelOrin/GOSpeak/commit/97f0d622b6713843d3ce540ee655cca4fc3ebce1))
* cluster agent runtime + observability stack + comprehensive test suite ([e33a9b7](https://github.com/NoelOrin/GOSpeak/commit/e33a9b7787fb72d4f77cd17d98406868dcd71071))
* cluster agent worker, room resolver and voice session updates ([7e32c93](https://github.com/NoelOrin/GOSpeak/commit/7e32c938673a55929fc4e5aabffc598f2b8a4da8))
* **cluster:** add cluster events, scheduler tests and handler APIs ([c6d972a](https://github.com/NoelOrin/GOSpeak/commit/c6d972a4e0ff6551cf43daf6a7f9ec349996059c))
* **cluster:** add leader lock and auto-scaling hooks ([67c2a7d](https://github.com/NoelOrin/GOSpeak/commit/67c2a7db40d7f463a21b7e615275c9861fa85e35))
* **cluster:** define NATS control command envelope ([2577e68](https://github.com/NoelOrin/GOSpeak/commit/2577e686905bbcda867d163e28d9e22d2062bd52))
* **cluster:** explicitly reject worker business writes ([1954bde](https://github.com/NoelOrin/GOSpeak/commit/1954bdea11daab3c7964620c772c3da63bc7e298))
* **cluster:** expose cluster health stats ([551e003](https://github.com/NoelOrin/GOSpeak/commit/551e003b2ce6383cc60181a20d4866ee1b310f88))
* **cluster:** publish control commands from agent ([140d0e0](https://github.com/NoelOrin/GOSpeak/commit/140d0e02cc0731af5c7d8a527f532e67e57bd1ee))
* **cluster:** reconcile cluster state on agent startup ([fa18537](https://github.com/NoelOrin/GOSpeak/commit/fa185370958a1a1397d126be1681d27ff13321b0))
* **cluster:** worker executes NATS control commands ([47e3b9d](https://github.com/NoelOrin/GOSpeak/commit/47e3b9da678eca3f4ac7f049d5335b4536338c1e))
* **cluster:** worker mode uses read-only DB and skips seeding ([a78c0b4](https://github.com/NoelOrin/GOSpeak/commit/a78c0b4ea9135e6d8994e1abdbb314a0520358b1))
* compact ([2363faf](https://github.com/NoelOrin/GOSpeak/commit/2363faf9f3b123c38c10af0c7ea49235e7b81a3b))
* **db:** support Turso libSQL with auto migration and tests ([e173dfb](https://github.com/NoelOrin/GOSpeak/commit/e173dfbbc77ed304759b8ffc88717d560d24dd5d))
* **deploy:** add cluster nginx routing ([10e04f4](https://github.com/NoelOrin/GOSpeak/commit/10e04f4f5241c703bc22d9aa568ddcdc20fd2c13))
* **domain:** add room management to domain admin page ([6878853](https://github.com/NoelOrin/GOSpeak/commit/68788533d950484b922d06fb00641184d026bf2b))
* **domain:** enrich member list with user names ([0697298](https://github.com/NoelOrin/GOSpeak/commit/0697298de79f50438ea6b969aac1c1f9c4b92464))
* **domain:** harden domain RBAC, mute, room and user management with db migration ([60d796e](https://github.com/NoelOrin/GOSpeak/commit/60d796eaaa7f3369f9597f7af76a93cf2e7a889e))
* **domain:** implement complete Domain module with CRUD and tests ([e1ee609](https://github.com/NoelOrin/GOSpeak/commit/e1ee609e301b23d60220ac2257ce16c8303ef8f8))
* **domain:** 事务化归属转移与跨实例踢出命令 ([c24548f](https://github.com/NoelOrin/GOSpeak/commit/c24548f582df48bb60781e9f0bdc0a05c29084d8))
* enforce domain permissions on message deletion ([e736183](https://github.com/NoelOrin/GOSpeak/commit/e736183a04df517907c1e03fe413118448771c0b))
* enhance message, auth, WebSocket and cluster capabilities ([8eb75aa](https://github.com/NoelOrin/GOSpeak/commit/8eb75aa7d580efb111277ec55a3a4bdd5753e370))
* **frontend:** update domain detail page with room improvements ([81da217](https://github.com/NoelOrin/GOSpeak/commit/81da2175267bc21643e22fdf42f48ea819298e3d))
* **frontend:** update socket store and API ([b6c826c](https://github.com/NoelOrin/GOSpeak/commit/b6c826c5bf1518860759c16f0b32e5856ba3c30c))
* **frontend:** update UI components, pages and assets ([b785f28](https://github.com/NoelOrin/GOSpeak/commit/b785f28cc489816a6bf4852262dd7bff0e2d5bd8))
* **frontend:** update UI components, pages and stores ([a0dabd1](https://github.com/NoelOrin/GOSpeak/commit/a0dabd1ebe02b52aded6f2c1bf619237e1406ca0))
* **guest:** 游客访问权限体系 + 登录权限服务重构 ([#17](https://github.com/NoelOrin/GOSpeak/issues/17)) ([33a9e61](https://github.com/NoelOrin/GOSpeak/commit/33a9e61d36af1f8dca7bd712081ac95a3b4ae467))
* **guild:** add default Guild migration for existing rooms ([edd6bbd](https://github.com/NoelOrin/GOSpeak/commit/edd6bbd69017e1e33e33037f4aeadf784449f7bf))
* **guild:** add frontend Guild API, store, components, and route page ([7f706c8](https://github.com/NoelOrin/GOSpeak/commit/7f706c882bf196220786074c4324f6e88d7a51cc))
* **guild:** add Guild and GuildMember data models ([0c1a84c](https://github.com/NoelOrin/GOSpeak/commit/0c1a84cc0ac3793aeadbfc9f8bae2c9292f39101))
* **guild:** add Guild handler, routes, middleware, and DI wiring ([206c0dd](https://github.com/NoelOrin/GOSpeak/commit/206c0ddbd2362f903faadab677eb24ba9a3e5d43))
* **guild:** add GuildRepository with CRUD and member operations ([a45563a](https://github.com/NoelOrin/GOSpeak/commit/a45563a2a07ec9d4463263e1b8007e4fd228c9ef))
* **guild:** add GuildService with create/join/leave/kick/transfer ([5cf9916](https://github.com/NoelOrin/GOSpeak/commit/5cf9916d68914409b56973220654d8bd408e820f))
* **guild:** add GuildUUID filtering to Room CRUD and signal roomStore interface ([dd5b626](https://github.com/NoelOrin/GOSpeak/commit/dd5b62625e0d73c457478fdd360aacedfc4b8cf4))
* **guild:** add GuildUUID to Room/Message models and guild permcodes ([8a7c758](https://github.com/NoelOrin/GOSpeak/commit/8a7c7588f9d52abb23681f06fd0c45c7332f71d8))
* **guild:** integrate GuildList into layout, add create/join buttons, enhance guild page ([daaa9d9](https://github.com/NoelOrin/GOSpeak/commit/daaa9d986cca01fee36097b4296ea2509b2b75e0))
* **guild:** namespace Signal Hub rooms by guild UUID ([b23d474](https://github.com/NoelOrin/GOSpeak/commit/b23d474facc8eb304c97b2379d6fd9dc6a77dd3e))
* **guild:** register Guild permissions in seed data ([0368ac9](https://github.com/NoelOrin/GOSpeak/commit/0368ac9a8fc4e25f5dfae8504ad1ddb6b9a356e0))
* **handler:** add domain-first resource permission helper ([c9dee97](https://github.com/NoelOrin/GOSpeak/commit/c9dee97932155ed52d93ee0439b4a951ca64b476))
* **im:** add message model and repository ([becfdbb](https://github.com/NoelOrin/GOSpeak/commit/becfdbb9a743ed021eedad105bf030ff0e0797b8))
* **im:** message list API and DI wiring ([10ad7ca](https://github.com/NoelOrin/GOSpeak/commit/10ad7ca5c135a5494bd53f379a6bddc498a1300f))
* **im:** message service with EventBus publish ([14726dd](https://github.com/NoelOrin/GOSpeak/commit/14726dd4038122a1bfcfdc1967b4a6cddd84c24e))
* **im:** socket message:send bridge via MessageService ([79428a0](https://github.com/NoelOrin/GOSpeak/commit/79428a09e878aeb3ef8eba9d5574881889a8d5a0))
* **mediasoup-worker:** 显式资源清理与单元测试 ([0f6a2a0](https://github.com/NoelOrin/GOSpeak/commit/0f6a2a039d65cdf1a169fb70c897152adf1128ad))
* **message:** enhance message and conversation services with tests ([353dcc3](https://github.com/NoelOrin/GOSpeak/commit/353dcc3be4c86363914a159f43927cba5112a634))
* **message:** 稳定作者 UUID、消息幂等与软删除状态 ([a05a8c3](https://github.com/NoelOrin/GOSpeak/commit/a05a8c3bb90cf9b968903294c095427e911a04e0))
* migrate and seed per-domain roles ([dd9a3a0](https://github.com/NoelOrin/GOSpeak/commit/dd9a3a00bea329172a000024d4092f686472c126))
* **model:** add conversation fields to Message and private chat event constants ([2e0c286](https://github.com/NoelOrin/GOSpeak/commit/2e0c286ea73a2c32d5f203fbf9d890446446b96a))
* **mute:** 临时禁言到期后完整解禁 ([721e9ae](https://github.com/NoelOrin/GOSpeak/commit/721e9aea89386975de5cb87c0a0998c9817a4309))
* **oauth:** add OAuth account encryption and provider models ([870ea7c](https://github.com/NoelOrin/GOSpeak/commit/870ea7cf0e6509e2d7101ce64127403b678bb88f))
* **oauth:** 回调 postMessage payload 携带 expires_in ([55d0c4c](https://github.com/NoelOrin/GOSpeak/commit/55d0c4c2110822d642afac32c0ca20efec6ed424))
* **room:** add domain-required validation and update room module ([6208649](https://github.com/NoelOrin/GOSpeak/commit/620864936c201d1aedd5c2fc5559bdd75654b96e))
* **room:** add room duplicate detection and list utilities ([8e0d628](https://github.com/NoelOrin/GOSpeak/commit/8e0d62879cee149f74c902726ae06fc54a167af2))
* **room:** improve room list and detail components ([11f780c](https://github.com/NoelOrin/GOSpeak/commit/11f780c7a6285aa346d32b0aeabd29f0bab75212))
* **server:** add guild middleware for request context and DB init improvements ([3e977a6](https://github.com/NoelOrin/GOSpeak/commit/3e977a6eb9ca02666779832f51f245c4c3a76edb))
* **server:** add guildUUID to JoinPolicy interface ([d7c20e4](https://github.com/NoelOrin/GOSpeak/commit/d7c20e4169f711f910ff998bd57ce6dc0cdb6201))
* **server:** complete guild permissions, discovery and room isolation ([abc10a5](https://github.com/NoelOrin/GOSpeak/commit/abc10a567de4d17c83e082fcdf0b0e253b896ab1))
* **server:** enforce room manage permission on update and delete ([76ab7d0](https://github.com/NoelOrin/GOSpeak/commit/76ab7d0e81f1b87f590d48b803bfab10eedcd72a))
* **server:** harden auth, uploads and WebSocket handshake ([1deb5ac](https://github.com/NoelOrin/GOSpeak/commit/1deb5ac75600cbfe14fc682e4d934e2f94d9a05a))
* **server:** reset-password CLI command and planning docs ([f26f326](https://github.com/NoelOrin/GOSpeak/commit/f26f3260d2c2987efb39229e7ff7c7445faca2f9))
* **server:** update room handler and service with guild isolation ([5e7ce20](https://github.com/NoelOrin/GOSpeak/commit/5e7ce20618525ca7dcae69d83d58eb1f78029850))
* **server:** 注入 bot token 校验、禁言过期与域踢出接线 ([6e4b8a5](https://github.com/NoelOrin/GOSpeak/commit/6e4b8a5b8ae56b26ee4a7e0e5b7e561e8f7a7770))
* **service:** add async write-steal pattern to message service ([320564f](https://github.com/NoelOrin/GOSpeak/commit/320564f2fee93a985db2eab9b04b6f6e47aa6fde))
* **service:** add SendDirect for private chat messages ([27ab316](https://github.com/NoelOrin/GOSpeak/commit/27ab31634e836ad73dd5b8972927e4691be12979))
* **sfu-client:** event-driven speaking detection ([1c0983b](https://github.com/NoelOrin/GOSpeak/commit/1c0983bcdef78655b27cdaba02d40a0a2c389ce4))
* **sfu-client:** implement restartIce in mediasoup client for transport reconnection ([64d5327](https://github.com/NoelOrin/GOSpeak/commit/64d5327273a20118af33a0eb9ad9528852619912))
* **sfu,srs:** Discord-style hard mute and SRS API room management ([74c835a](https://github.com/NoelOrin/GOSpeak/commit/74c835afd465e3efb0635d012c1ecf63026972c3))
* **sfu:** consolidate hard mute rule store across sfu/bus and agora/srs providers ([972b93c](https://github.com/NoelOrin/GOSpeak/commit/972b93ca31de25bef7a3565097f60483d3090d1a))
* **sfu:** hard mute fixes, provider hardening and UI contract alignment ([f61a2ba](https://github.com/NoelOrin/GOSpeak/commit/f61a2ba65fae62aa22a05c5e3e07f497baea2360))
* **sfu:** 管理页按 provider 隔离保存配置 ([53cdca7](https://github.com/NoelOrin/GOSpeak/commit/53cdca7b36dad874b6da340496fd39a49981b645))
* **sfu:** 能力矩阵覆盖、provider 资源释放与错误测试 ([d5b34c8](https://github.com/NoelOrin/GOSpeak/commit/d5b34c8e5016b94830407fd88238a2516adc3213))
* **signal:** add ByIdentity index for O(1) member lookup in Hub ([8342346](https://github.com/NoelOrin/GOSpeak/commit/83423466068584b86662219cca1f6db08619da93))
* **signal:** add KV membership fallback to message bridge ([c68321c](https://github.com/NoelOrin/GOSpeak/commit/c68321cd45fa9b8cf7b9721371004a617df97897))
* **signal:** add OnGuildDelete cleanup and DRY test setup ([104fc74](https://github.com/NoelOrin/GOSpeak/commit/104fc7446b0fa41ccefe898781dff75c196d51a5))
* **signal:** update hub with guild namespace isolation and state sync ([ed6c17f](https://github.com/NoelOrin/GOSpeak/commit/ed6c17f8ef257e6c5af46a1b1bda61c42e165e3d))
* **signal:** wire private:send WS event and /conversation/send endpoint ([c20859b](https://github.com/NoelOrin/GOSpeak/commit/c20859b88b28e7dec7c5ef7f4b9c4bbd6d798f67))
* **signal:** 域内踢人与踢出冷却，join 注册移出全局锁 ([16ff5a3](https://github.com/NoelOrin/GOSpeak/commit/16ff5a30941b5a2879a052bf149047381cdaccd5))
* **signal:** 多实例房间元数据与成员原子注册 ([a0e4803](https://github.com/NoelOrin/GOSpeak/commit/a0e4803d97d3454b0cf87b4bd92cba5ddfdd9604))
* **skills:** update room-voice-e2e skill documentation ([e3a47f8](https://github.com/NoelOrin/GOSpeak/commit/e3a47f8da4bb5ee24d5bd546c054a0c2f0106736))
* socket ([a2a68a5](https://github.com/NoelOrin/GOSpeak/commit/a2a68a587643164c2cc1e304f55d08366e3e3a20))
* **socket:** add native WebSocket client adapter ([277ff18](https://github.com/NoelOrin/GOSpeak/commit/277ff18c01817b3a0270f1b68d89ec33bc4a6feb))
* **socket:** bind PRIVATE_NEW event globally in socketStore ([e9a33d7](https://github.com/NoelOrin/GOSpeak/commit/e9a33d71f051aedfa126d703cb5f703172a5e4b9))
* **storage:** allow PDF and plain text uploads ([4853be2](https://github.com/NoelOrin/GOSpeak/commit/4853be2c12ca0f1327a02f59dc357f9150fd5fb8))
* **user-group:** add user group module (backend + frontend) ([77d7c6d](https://github.com/NoelOrin/GOSpeak/commit/77d7c6d761cf5960171dfed2807168acff5b3d61))
* **web:** add cluster management page ([20e4c6b](https://github.com/NoelOrin/GOSpeak/commit/20e4c6be7c49a0ed99daeeec63a389d7b2bc7341))
* **web:** add domain room table ([e5aaf38](https://github.com/NoelOrin/GOSpeak/commit/e5aaf38dbc0b18e2856bd23a5e8135d50b79fb90))
* **web:** add edit room modal ([9735fc9](https://github.com/NoelOrin/GOSpeak/commit/9735fc968a2ec1699072724172da159a6861bc5d))
* **web:** add guild discovery, text chat and UX improvements ([151add8](https://github.com/NoelOrin/GOSpeak/commit/151add842273fce3c96807890c4f0144cb78e9b7))
* **web:** add guild invite preview and polish invite flow ([035ae25](https://github.com/NoelOrin/GOSpeak/commit/035ae2538312f002eaea4b596309565e32879f27))
* **web:** add GuildMemberTable component and guild management page ([02d4323](https://github.com/NoelOrin/GOSpeak/commit/02d432345d7583dc0445df9e0b55f2bfdeb42165))
* **web:** add PIP keepalive utility for iOS picture-in-picture ([763a342](https://github.com/NoelOrin/GOSpeak/commit/763a3424c3faed453832a27278f7cffbbc2a0ab3))
* **web:** add room list update delete api ([062c8e7](https://github.com/NoelOrin/GOSpeak/commit/062c8e7070d0ab8e84116c26886ea491020c6172))
* **web:** APIKey 吊销改用确认弹窗 ([50e4d9c](https://github.com/NoelOrin/GOSpeak/commit/50e4d9cfb809805eabacc992a52280c3e14c7046))
* **web:** domain invite share modal and link page ([dcc26d5](https://github.com/NoelOrin/GOSpeak/commit/dcc26d5b51384789931b51b64f217942924c346a))
* **web:** extend PIP keepalive for AndroidTWA and improve error handling ([2d69006](https://github.com/NoelOrin/GOSpeak/commit/2d690060f319d6b8b6c9ba18ccd3a3b2d84b952b))
* **web:** pass explicit domain when creating room ([bb1c225](https://github.com/NoelOrin/GOSpeak/commit/bb1c22593995d218b5330f6f19b4cd70477deea7))
* **web:** polish room and domain UI ([b33d509](https://github.com/NoelOrin/GOSpeak/commit/b33d509da24dd9b774df9fa0a1dd33f6c8b90e6a))
* **web:** private message API types and socket events ([2073b70](https://github.com/NoelOrin/GOSpeak/commit/2073b7013dae35e8b78c7126501da7f692abe982))
* **web:** room list type icons per room type ([5b15cc7](https://github.com/NoelOrin/GOSpeak/commit/5b15cc778d92ebbe110dff88cfe7c1faf5791a08))
* **web:** route voice signal join to assigned worker ([f13b10b](https://github.com/NoelOrin/GOSpeak/commit/f13b10b3d35205a42cacbcacf2fa85b80f66337c))
* **web:** Socket 状态机、乐观消息幂等与断线清理 ([bc33270](https://github.com/NoelOrin/GOSpeak/commit/bc3327077cd3a870d8f48d2fafbaa752d18f80a3))
* **web:** support target domain in create room modal ([99f3e6c](https://github.com/NoelOrin/GOSpeak/commit/99f3e6cc3d57beea0f7915f5a7360eb320a7e0bf))
* **web:** support worker URL socket connection ([434b01a](https://github.com/NoelOrin/GOSpeak/commit/434b01ae3bff1374f7fcd15d953ac3d71240f58e))
* **web:** update guild store and API with multi-guild support ([30a6a83](https://github.com/NoelOrin/GOSpeak/commit/30a6a83b05dfe8cd3f9411854280f641f0e03509))
* **web:** update room list UI, guild index page, and room state management ([f5c786e](https://github.com/NoelOrin/GOSpeak/commit/f5c786e53811504c39cf79c961e3ce3820114887))
* **web:** 邀请落地页与注册页改为独立路由 ([803e51d](https://github.com/NoelOrin/GOSpeak/commit/803e51d043459f0874dd491ac3c90f6b6e3549d1))
* **ws:** add ws package (Client, Fanout, HandlerRegistry, Upgrader) + WSDeliverer ([da0165d](https://github.com/NoelOrin/GOSpeak/commit/da0165d57f38e2348f71196d2e13215f47f5ab8d))
* **ws:** complete socket.io to WebSocket migration ([d632ef2](https://github.com/NoelOrin/GOSpeak/commit/d632ef2917edaa8c444931f233cb725ed8f2cedb))
* **ws:** 连接状态机与错误 ACK 脱敏 ([1f91110](https://github.com/NoelOrin/GOSpeak/commit/1f91110e17479710bc1c1110e2a00fec7c131c3f))
* 域权限优先并下线全局权限页 ([f7adcac](https://github.com/NoelOrin/GOSpeak/commit/f7adcac4d1d180b4cae3c0db034706f3466ba8f7))
* 添加文字聊天功能并完善相关文档与接口 ([cd58724](https://github.com/NoelOrin/GOSpeak/commit/cd58724612af367e442042aa1f962c8b9cadd5e1))


### Bug Fixes

* **android:** robust startForeground with try/catch and logging ([60fd1e8](https://github.com/NoelOrin/GOSpeak/commit/60fd1e83e82fb10c13e8eb140cb4c5875c3ae1c7))
* **auth:** revoke HttpOnly refresh cookie on logout ([b039061](https://github.com/NoelOrin/GOSpeak/commit/b0390619cd6e802f55856919da3097585501172e))
* **auth:** rotate refresh family on every refresh ([2ccabf1](https://github.com/NoelOrin/GOSpeak/commit/2ccabf15ff93742b62b72464dbf71b687ee42a74))
* **auth:** 开发态固定 JWT 静态密钥 ([dc85ba3](https://github.com/NoelOrin/GOSpeak/commit/dc85ba3d84436273b21a9611b08daa1d63d8a716))
* **bot:** add livekit-client type declaration for optional dependency ([1ba8b12](https://github.com/NoelOrin/GOSpeak/commit/1ba8b12ebcc58864e95cc36005a5d31e5e482ceb))
* **bus:** sanitize colons in NATS KV keys ([d756737](https://github.com/NoelOrin/GOSpeak/commit/d75673737991555851c646aa19b0e06850a3b28c))
* **chat:** set author_id on optimistic messages and remove duplicate PM listeners ([ad7452d](https://github.com/NoelOrin/GOSpeak/commit/ad7452d0dd1d633f9e30dd2687a86f4e55824e24))
* CI 修复与 v0.2.3 发布改动 ([76b53c3](https://github.com/NoelOrin/GOSpeak/commit/76b53c320a66b89f3132af517e3dd62d878be76e))
* **ci:** build docker image and cross-platform binaries on release: published ([979983f](https://github.com/NoelOrin/GOSpeak/commit/979983fe556483491f7daf0cb53309ddc9ea54f4))
* **cluster:** 内嵌总线跳过选主与写面 fence ([e615e8b](https://github.com/NoelOrin/GOSpeak/commit/e615e8bb5e4ea78d338f4e91f1f186943ff93053))
* compilation blockers, panic risks, crypto, SFU provider bugs ([0d136cb](https://github.com/NoelOrin/GOSpeak/commit/0d136cbf9bbcd63d4f3770432f01d2c55ebcbd1c))
* **docker:** copy local go patch before mod download ([d087560](https://github.com/NoelOrin/GOSpeak/commit/d08756081c3eda88212d0714cca8ad03ec961e55))
* **domain:** restrict admin assignment to domain owner ([b4ca9ac](https://github.com/NoelOrin/GOSpeak/commit/b4ca9ac10935010b10bff128058282b383ad78a8))
* **domain:** 创建语音服务器时播种系统角色 ([c08a402](https://github.com/NoelOrin/GOSpeak/commit/c08a402ef2c6dbee4c78e69100542057a598e637))
* **domain:** 启动回填存量域系统角色 ([73f5c6b](https://github.com/NoelOrin/GOSpeak/commit/73f5c6b95606f7487a268e7a8a6b3a6c6bcaea38))
* **guild:** add error handling, loading text, and data-ready guard to guild page ([40bfa7c](https://github.com/NoelOrin/GOSpeak/commit/40bfa7c6bbb6e5c2ecb0c7910328b339c8dd4704))
* **handler:** apply claims-aware checks to room manage and delete others ([2eb09fe](https://github.com/NoelOrin/GOSpeak/commit/2eb09fe39b8416a2f2ad9626d3ff0fd8bc3e36b2))
* **handler:** guard domain permission helper against typed nil checkers ([01fefc4](https://github.com/NoelOrin/GOSpeak/commit/01fefc4b085521d817b128f792f6c4b35b5f0b04))
* **handler:** honor explicit claims permissions on platform fallback ([fdf4d40](https://github.com/NoelOrin/GOSpeak/commit/fdf4d403b2c65e8a76a8b3c2efc3f30afd892334))
* **handler:** monitor 与 mute 处理器边界条件修复 ([fea1938](https://github.com/NoelOrin/GOSpeak/commit/fea19389e4a3710d89acada0abb2c49b4b7e64ac))
* **handlers:** 控制面失败不阻塞业务响应并拒绝 SRS 非法回调 ([1d46b68](https://github.com/NoelOrin/GOSpeak/commit/1d46b687ae30ad50a089c379f9429fe82793c065))
* harden cluster/signal/web/deploy review findings ([b481e3f](https://github.com/NoelOrin/GOSpeak/commit/b481e3fdba0a1e4096c6e7fdb51804362f81ff7d))
* heartbeat goroutine exit wait + S3 5s timeout context ([ed0e470](https://github.com/NoelOrin/GOSpeak/commit/ed0e47005d0e1c1481a611c5a6acf897d726f342))
* **im:** address review findings ([8d45737](https://github.com/NoelOrin/GOSpeak/commit/8d457377228df8f6b9f0013027abfe5c4b295aba))
* **im:** 修未读数bug + fanout nil保护 + 消息竞态 + ack/事务/nit ([dd3c336](https://github.com/NoelOrin/GOSpeak/commit/dd3c336b2c9b04af5a1c51ecbf35485bd1764920))
* mediasoup restart-ice, form type safety, domain_uuid validation, permissions from profile ([3ed950e](https://github.com/NoelOrin/GOSpeak/commit/3ed950eead5c7408b6fa93d45a8d3d0f525f0b8a))
* **message:** add write-worker fallback and graceful shutdown ([9d8e757](https://github.com/NoelOrin/GOSpeak/commit/9d8e75753dd08ba73ba892a7e1e6f899e08832bc))
* **message:** enforce domain role permissions on message endpoints ([10edb3e](https://github.com/NoelOrin/GOSpeak/commit/10edb3e657e97c06bec64751682135ff53c05c53))
* **message:** 实时广播 DTO 携带 author 富字段 ([ce1eef6](https://github.com/NoelOrin/GOSpeak/commit/ce1eef698c61612d0e4bd5244d44a7de8a5c9ded))
* **ops,signal,web:** cherry-pick portable fixes from mobile-responsive branch ([ef995aa](https://github.com/NoelOrin/GOSpeak/commit/ef995aaae051f4ede5eb994bf092b62283a8896e))
* **packages:** bot permissions, mediasoup dedup, srs cleanup ([3d7cc75](https://github.com/NoelOrin/GOSpeak/commit/3d7cc75425b5b712a5ec91cf733269fa19365092))
* **plugin:** TOCTOU 竞争修复与注册逻辑完善 ([cfa3261](https://github.com/NoelOrin/GOSpeak/commit/cfa32611baaa7395b534b437230843029b248841))
* **plugin:** 拒绝非法生命周期状态迁移 ([6ea4348](https://github.com/NoelOrin/GOSpeak/commit/6ea43487d7b75bc0b3abf487f9517b4021df7254))
* remove duplicate ConversationList import + add @tanstack/solid-virtual ([b59db15](https://github.com/NoelOrin/GOSpeak/commit/b59db15a419aef6418de5112952b2f61efb4382e))
* review gap fixes ([6ba3dfb](https://github.com/NoelOrin/GOSpeak/commit/6ba3dfb5c00d88840f3578d079fdb14666f849de))
* **room:** enforce domain role permissions on room endpoints ([4154891](https://github.com/NoelOrin/GOSpeak/commit/4154891a274f93592b2b5355415435514484a45c))
* **room:** enforce resource permissions in handler for all room routes ([696cccc](https://github.com/NoelOrin/GOSpeak/commit/696ccccfa4b1312d1eee62dab3d5e4d0be931ac9))
* **room:** let domain roles manage rooms without global permission gate ([0ce9e72](https://github.com/NoelOrin/GOSpeak/commit/0ce9e7234305a54420df878cb13d021a393b5550))
* **room:** remove unused voiceChat component ([02a0de8](https://github.com/NoelOrin/GOSpeak/commit/02a0de8a06b6338fe3bdbc91aba7fce9ecc51580))
* **router:** normalize import ordering and fix test coverage ([4c79a0d](https://github.com/NoelOrin/GOSpeak/commit/4c79a0ddee4e09278cf0602695ca7c3988da54ea))
* **server:** bind cloudflare sessions to their owner ([38a6b1e](https://github.com/NoelOrin/GOSpeak/commit/38a6b1e8158e80634ee4fc88086251f07ff54f1d))
* **server:** concurrency safety for role cache and job queue ([c22900a](https://github.com/NoelOrin/GOSpeak/commit/c22900a79de447b01afb95d79bfa1c6ab9561c2f))
* **server:** enforce domain membership and use composite sfu room ([9677ed9](https://github.com/NoelOrin/GOSpeak/commit/9677ed97ecb3f68d4017d9a1d6b837d8af481e2b))
* **server:** harden cluster lifecycle, key rotation and state sync ([ba61dd2](https://github.com/NoelOrin/GOSpeak/commit/ba61dd21635e529d9892b1e28ddd6d466ae09a73))
* **server:** harden oauth bot plugin auth and secret storage ([8e84bfd](https://github.com/NoelOrin/GOSpeak/commit/8e84bfd21680d5e558dd3db348fdb9c246f62607))
* **server:** harden ws lifecycle and speaking mute checks ([491815c](https://github.com/NoelOrin/GOSpeak/commit/491815c18dd19c644fdccb3957a6790a9dfadef5))
* **server:** keep permanent mutes enforced beyond 24 hours ([62ac19c](https://github.com/NoelOrin/GOSpeak/commit/62ac19c4ee0c08973b56dc8eb1aa8a8e69a729e8))
* **server:** path traversal protection and nil pointer fixes ([adfa5a9](https://github.com/NoelOrin/GOSpeak/commit/adfa5a97b8f3ec0f20823ba7d192076c1799b7cc))
* **server:** race-safe guild checker, stack-allocated invite code, testutil options ([2d375fc](https://github.com/NoelOrin/GOSpeak/commit/2d375fc0f9cb02cee5a82bafa8e2bd7af701161c))
* **server:** read domain_uuid from middleware context ([5d371b8](https://github.com/NoelOrin/GOSpeak/commit/5d371b89d96abaf4325973a02e5805713100de8e))
* **server:** reload storage config and enforce upload ownership ([ed30b18](https://github.com/NoelOrin/GOSpeak/commit/ed30b184fb74caada39d5a5049cdf31d560c582b))
* **server:** repair upload ownership check and plugin secret encryption ([2acca54](https://github.com/NoelOrin/GOSpeak/commit/2acca54b481f3883e195b3ab4316db4d4256485c))
* **server:** require domain on room create and broadcast room list ([29e62f2](https://github.com/NoelOrin/GOSpeak/commit/29e62f2bad83707ad7259a9d2ee2342502ae4385))
* **server:** verify webhook signatures and honor bot claim permissions ([9925756](https://github.com/NoelOrin/GOSpeak/commit/9925756e5b75098f1af55acbe5695ac78a03698e))
* **service:** 错误处理统一与边界检查完善 ([c1ba193](https://github.com/NoelOrin/GOSpeak/commit/c1ba193a30a4e8490dd1f5487064ba248420b38c))
* set cookie Secure flag based on TLS to avoid cookie issues in prod ([7100429](https://github.com/NoelOrin/GOSpeak/commit/7100429a31a9c7ae45af3e65b69c459322559ae9))
* **sfu-client:** match domain-scoped room events ([9ca8c65](https://github.com/NoelOrin/GOSpeak/commit/9ca8c652eb99985509eae7784b8a950b3dc45804))
* **signal:** deny ws message access without claims ([c649b00](https://github.com/NoelOrin/GOSpeak/commit/c649b00848780f5dd0db3d6a150ed3fa84c36e39))
* **signal:** enforce domain role permissions on ws message events ([6a3a6ac](https://github.com/NoelOrin/GOSpeak/commit/6a3a6ac8f34c71ea88560f0131d1aca0e1ff2fdd))
* **signal:** fail closed when ws domain permission checker missing ([0ad7d30](https://github.com/NoelOrin/GOSpeak/commit/0ad7d30234ca05941ec884e16ef67ae0e874a0c4))
* **signal:** use composite room keys for media cleanup and slots ([09caf2e](https://github.com/NoelOrin/GOSpeak/commit/09caf2eba1556b00ca22fbe72e207f27ccf605ce))
* **text-room:** improve message input and rendering ([111b4ae](https://github.com/NoelOrin/GOSpeak/commit/111b4ae835d5f77e30c7676b23e3e6a556044ad4))
* **web:** correct edit room modal import casing ([d3505f5](https://github.com/NoelOrin/GOSpeak/commit/d3505f5088a42e380ccdd90f1d48e6f9f0648c1c))
* **web:** form validator, UserInfo type, and vite config guard ([82266af](https://github.com/NoelOrin/GOSpeak/commit/82266af388e2490707ee91c36be96adf15367f78))
* **web:** pass domain_uuid and sfuRoom through voice join ([7d10aab](https://github.com/NoelOrin/GOSpeak/commit/7d10aabbd62e0d6b92bf9f3421e5d898ce82747d))
* **web:** room list scrolling with hidden scrollbar ([d7e8b94](https://github.com/NoelOrin/GOSpeak/commit/d7e8b9473d9e2789b033b7c49162da896d62b2db))
* **web:** surface edit room errors and unify validation ([e47db04](https://github.com/NoelOrin/GOSpeak/commit/e47db04f5ac0c3dea05008a6c486463220af32b0))
* **web:** sync frontend permissions and update AGENTS docs ([0cf6a60](https://github.com/NoelOrin/GOSpeak/commit/0cf6a60ecbf55da32d74cd0876df53a381e2e751))
* **web:** 根治路由切换闪烁的三处残留根因 ([4293217](https://github.com/NoelOrin/GOSpeak/commit/429321772ae3329229a0e55ab155b8524bf83006))
* **web:** 移除管理页入场动画与路由多余 default export ([07d825a](https://github.com/NoelOrin/GOSpeak/commit/07d825a92dd38c059e3339d70839e0c24952cd90))
* **web:** 重连成功后关闭 reconnecting toast ([fdb9583](https://github.com/NoelOrin/GOSpeak/commit/fdb9583c89fa761328e29ee3088720e54c740d13))
* **web:** 音量控件竖向滑条重绘与弹层对齐 ([a85f1fb](https://github.com/NoelOrin/GOSpeak/commit/a85f1fb2815793885dc6918f935760d2e3035c8b))
* **ws:** 连接级唯一 ID、心跳保活与幂等关闭 ([6789cd8](https://github.com/NoelOrin/GOSpeak/commit/6789cd823459e32fde0e519d97a421cdffea6e02))
* 修复跨域房间切换与 chatStore 循环依赖 ([cd14817](https://github.com/NoelOrin/GOSpeak/commit/cd14817019feb714280b9822dc5e5099e3484108))
* 小问题 ([316bae4](https://github.com/NoelOrin/GOSpeak/commit/316bae402fda5c139e6b4b14858902e080b61ad5))


### Performance

* **signal:** batch localRoomSnapshots to eliminate N+1 in getMergedRoomsScoped ([8ec5eca](https://github.com/NoelOrin/GOSpeak/commit/8ec5ecab97ca3e5af344b526b25e8eacbaaba0ce))


### Documentation

* add bilingual changelog baseline ([27c4c32](https://github.com/NoelOrin/GOSpeak/commit/27c4c32805e41591838bd841e46b188bada3ed3f))
* add tauri mobile hybrid webrtc design and plan ([14474a8](https://github.com/NoelOrin/GOSpeak/commit/14474a89bff9c26bb2beb5c1b21f7259fb72e06d))
* add websocket fanout migration plan ([7989837](https://github.com/NoelOrin/GOSpeak/commit/7989837da2c4b105a9ebec622b460ec64782a780))
* auto-generate changelog for 0.2.1 and make release include changelog ([4d4163a](https://github.com/NoelOrin/GOSpeak/commit/4d4163a948cf12a5bf1b641ec3fab741cbc4d95c))
* **cluster:** mark control plane completion status ([826634b](https://github.com/NoelOrin/GOSpeak/commit/826634b0d7ee3dab790b5069d7a8aaf93148930b))
* **config:** update AGENTS.md, vite config and frontend documentation ([0f2ea13](https://github.com/NoelOrin/GOSpeak/commit/0f2ea133587749ddc3695e5b83a869141b9d63f7))
* design domain room management feature ([c4eafab](https://github.com/NoelOrin/GOSpeak/commit/c4eafab1130f4388a854350f42e67dc267e74265))
* expand cluster agent worker plan with discovery phase ([f51a3b2](https://github.com/NoelOrin/GOSpeak/commit/f51a3b2841b2f7e064b5b84b6dda654cb9f027cb))
* **guild:** update AGENTS.md with Guild architecture and route table ([45c2c1f](https://github.com/NoelOrin/GOSpeak/commit/45c2c1fbb2403d1e580536f2d50d65ab52675add))
* mark cluster-agent-worker plan as delivered with verification evidence ([dcfca84](https://github.com/NoelOrin/GOSpeak/commit/dcfca8405fdc92a2a6056dbd85fc35418bb55327))
* **plans:** room IM over EventBus implementation plan ([2bb880e](https://github.com/NoelOrin/GOSpeak/commit/2bb880e360cd1129dc330a582b26993b9d128310))
* README ([69f846d](https://github.com/NoelOrin/GOSpeak/commit/69f846d447823b7eebc4de569d6472939f2f9571))
* **review:** mark repo hygiene items as handled ([30e8c13](https://github.com/NoelOrin/GOSpeak/commit/30e8c13b8b5dae224db4d4b1f26022a4e7c61ab6))
* **review:** split p0 auth and domain rbac plans ([9e5051a](https://github.com/NoelOrin/GOSpeak/commit/9e5051a05dcc4005df7c27039a0c3e1073326f22))
* sync review status and gap-fix plan completion ([cfd4a77](https://github.com/NoelOrin/GOSpeak/commit/cfd4a77100ad671b998c870ac6b431dfd9aba89a))
* update AGENTS and swagger for domain rename ([4b711cb](https://github.com/NoelOrin/GOSpeak/commit/4b711cba0372afe7f388115e1cd53c090a6cd9f1))
* update AGENTS.md across monorepo for new modules ([f043400](https://github.com/NoelOrin/GOSpeak/commit/f043400ea25d8f9c6dc8a272ae15094996090ffb))
* update AGENTS.md and documentation ([8ec79c9](https://github.com/NoelOrin/GOSpeak/commit/8ec79c9fa4f23631e8e2b33771b53774abbc5858))
* update AGENTS.md and regenerate swagger docs ([305d5e0](https://github.com/NoelOrin/GOSpeak/commit/305d5e07b470c36d78372677cfe95a01ff5c2860))
* update deployment docs and superpowers specs/plans ([6466b3d](https://github.com/NoelOrin/GOSpeak/commit/6466b3d198d034b1360c118045d01946d9decc81))
* **web:** define room and domain UI ([bd94da9](https://github.com/NoelOrin/GOSpeak/commit/bd94da93c95f23a045629c94a1b77f58fb72f2df))
* 会话主动续期（expires_in + 懒续期）实现计划 ([3ac75bf](https://github.com/NoelOrin/GOSpeak/commit/3ac75bfae97a02beba68df1764e14c373b663a66))
* 会话过期下发与主动续期（expires_in + 懒续期）设计规格 ([5d4d00c](https://github.com/NoelOrin/GOSpeak/commit/5d4d00c5f05ac994fa438155b8b5307abdc168be))
* 同步 Domain 术语与多实例状态同步说明 ([70d28e2](https://github.com/NoelOrin/GOSpeak/commit/70d28e2b09864e5ce49b16d52f4be54500dc1c30))
* 按代码现状全量修订 AGENTS.md ([ddd5156](https://github.com/NoelOrin/GOSpeak/commit/ddd5156da32d2d0d763027eb0137c34884d3efe6))
* 更新站点资源与文档，替换svg资源为png格式 ([d2d0e9d](https://github.com/NoelOrin/GOSpeak/commit/d2d0e9dd6e66cf6f976196fd9ee720c7794a120a))
* 补充 v0.3.0 changelog（游客访问权限体系 + 登录权限重构） ([47a6592](https://github.com/NoelOrin/GOSpeak/commit/47a659280301fb3bfd547d3bfa4238d1d84069a6))
* 配置文档部署基础路径与CI触发规则 ([#9](https://github.com/NoelOrin/GOSpeak/issues/9)) ([a3d797a](https://github.com/NoelOrin/GOSpeak/commit/a3d797a91d22612be06e3d4d95b2c9e7566e4b3f))


### CI/CD

* add release-please automated tagging ([5407221](https://github.com/NoelOrin/GOSpeak/commit/540722199c7f0e0fd44025307cc19668996fbfdd))
* CI 与提交链路优化（release-please 攒批发版） ([#18](https://github.com/NoelOrin/GOSpeak/issues/18)) ([8ab002d](https://github.com/NoelOrin/GOSpeak/commit/8ab002d035c3cf36155a09de587b92df7476aa4b))
* cross-compile release binaries on ubuntu ([0ec72d5](https://github.com/NoelOrin/GOSpeak/commit/0ec72d51438775784012807f13abc79dd2e9c68e))
* **docs:** deploy on release only if docs changed ([256c044](https://github.com/NoelOrin/GOSpeak/commit/256c044b20419531b775fc38d1d9cf96eb9ea728))
* **docs:** only deploy when docs change ([9bd0653](https://github.com/NoelOrin/GOSpeak/commit/9bd06530faff2eac50e1a9f45832530cdf57a859))
* **docs:** redispatch release deploy from branch ([3b71645](https://github.com/NoelOrin/GOSpeak/commit/3b71645701a8444aad8db3092a655769dc193e4e))
* fix workflow ([#6](https://github.com/NoelOrin/GOSpeak/issues/6)) ([9bdec22](https://github.com/NoelOrin/GOSpeak/commit/9bdec227de882ebfd3f3e42bb61d01eaae46828a))
* **github workflows:** 修复报错 ([#8](https://github.com/NoelOrin/GOSpeak/issues/8)) ([3423cb1](https://github.com/NoelOrin/GOSpeak/commit/3423cb1c4a2226eca20068368dc054e18370e960))
* merge release-please automated tagging ([f1d2030](https://github.com/NoelOrin/GOSpeak/commit/f1d20305dc7117d2a45bb73ae79fde3b0a6eb746))
* **release:** 新增 create-release job，手动发版时自动生成 Release 说明 ([a680ab0](https://github.com/NoelOrin/GOSpeak/commit/a680ab0f5ea3223aea011484dd71f761b7e9142f))
* rewrite release workflow for auto-release on push to branch ([c3f9a9d](https://github.com/NoelOrin/GOSpeak/commit/c3f9a9d7f9100ef856a7c80ffb518d6599391d18))
* split build/release workflows; fix cloudflare test race and Windows cross-build ([57fe468](https://github.com/NoelOrin/GOSpeak/commit/57fe468e13c24f09447f64b6e3a23e7c6bd6e8b6))
* split workflows and enforce versions ([67f8898](https://github.com/NoelOrin/GOSpeak/commit/67f88986b1c365140293b5945df25e1e13489483))
* start releases from v0.2.0 ([c945f37](https://github.com/NoelOrin/GOSpeak/commit/c945f37e8e339b211a8718740e8bd7f9767ab6fd))
* 修复 Version Consistency 在分支 push 时的假阳性失败 ([8aacd5d](https://github.com/NoelOrin/GOSpeak/commit/8aacd5d8aa4bb085f5600f4116b21263d570f5b2))
* 升级 GitHub Actions 到 Node 24 兼容版本，消除弃用告警 ([354d552](https://github.com/NoelOrin/GOSpeak/commit/354d552b54d19232516e9b915555ea52f66a3a3a))
* 升级 upload/download-artifact 到 v7/v8 消除剩余 Node 20 告警 ([c0a721c](https://github.com/NoelOrin/GOSpeak/commit/c0a721c3deb0afeff0e00079516844ed4180c218))


### Refactoring

* **auth:** integrate casbin, authboss, go-oauth2 and go-mail ([1a07f47](https://github.com/NoelOrin/GOSpeak/commit/1a07f47700a28ff3db6152802049a6c59fcfef4f))
* **infrastructure:** update config, router, WS and server setup ([5358e34](https://github.com/NoelOrin/GOSpeak/commit/5358e34e4fee94cfcdeb3160ffdbc617ecc8ffae))
* **plugin:** 拆分 botbase 密钥处理 ([153c30e](https://github.com/NoelOrin/GOSpeak/commit/153c30e87491f5dd0b8e05a95cdec45602548f51))
* remove direct Redis dependency in favor of NATS KV ([237e3d5](https://github.com/NoelOrin/GOSpeak/commit/237e3d51243dd1b7789f76f5a3b0d49ff8e2c7c6))
* **server:** rename guild to domain and migrate to cuid2 ([f98f701](https://github.com/NoelOrin/GOSpeak/commit/f98f701a8083541a265ad41824533838911ea465))
* **server:** replace socket.io with ws.Fanout and Upgrader in gin.go ([da92c34](https://github.com/NoelOrin/GOSpeak/commit/da92c34cc5fa0ddf95b277d010e03d1936ca31af))
* **server:** server wiring and graceful shutdown ([724a290](https://github.com/NoelOrin/GOSpeak/commit/724a2904bb0816596c059c5910adac705be8e44d))
* **services:** update conversation and message service implementations ([a5fefdc](https://github.com/NoelOrin/GOSpeak/commit/a5fefdc6d3f97d5d437b5cf9768e2cdc43847414))
* **sfu:** disable mediasoup and daily providers and sync docs ([8502765](https://github.com/NoelOrin/GOSpeak/commit/85027659588810ee22a478954ef2abf84c113f43))
* **signal,bus:** 批量查询优化与集群锁增强 ([86679e8](https://github.com/NoelOrin/GOSpeak/commit/86679e8917753b60083919a67a65c81fc480ff40))
* **signal:** replace socket.io with ws.Broadcaster and ClientMessenger ([768d97d](https://github.com/NoelOrin/GOSpeak/commit/768d97dcdd5646056744646b7eed8b72593a9a5f))
* **signal:** websocket hub 重构与 state sync 优化 ([beded3f](https://github.com/NoelOrin/GOSpeak/commit/beded3f830d651edd7bb0e76148b90e8dfcc3f87))
* split large backend and frontend modules ([86879ff](https://github.com/NoelOrin/GOSpeak/commit/86879ff09313615723d55e2b31b8d9b3a048f522))
* split large files into focused modules ([5575441](https://github.com/NoelOrin/GOSpeak/commit/55754415a5bd9bccde88b552833b71163428f62b))
* **web:** merge guild rail into sidebar ([9ea5721](https://github.com/NoelOrin/GOSpeak/commit/9ea5721f4a24fbb6ab811c864924ba316e35f83e))
* **web:** rename guild to domain across stores, api, and UI ([6222ca4](https://github.com/NoelOrin/GOSpeak/commit/6222ca4454baa97ddb7b8ed2f5ccdf28aa9e38b1))
* **web:** unify API error toasts in response interceptor ([39f494a](https://github.com/NoelOrin/GOSpeak/commit/39f494aa8bcb0b3ddca31de883b8a2b15d0d9051))
* **web:** 图标全量迁移 lucide-solid ([1e24e9a](https://github.com/NoelOrin/GOSpeak/commit/1e24e9a3cb5acdf23d654d272d18cde8a402e8c9))
* 完善多域名房间切换的域信息传递与处理逻辑 ([88bfb96](https://github.com/NoelOrin/GOSpeak/commit/88bfb960328880b6d90565e457a83516da1dd221))
* 清理废弃代码并重构表单校验逻辑 ([ef82964](https://github.com/NoelOrin/GOSpeak/commit/ef82964fd41ca257ea66b1498fc5a88ceaa3db4d))
* 重构为单二进制打包，内嵌前端资源 ([5b9eca7](https://github.com/NoelOrin/GOSpeak/commit/5b9eca7f35c7b5c61ec37bbfdfd5c0c2ddccb216))
* 重构房间列表与域相关逻辑，优化用户体验 ([01d5782](https://github.com/NoelOrin/GOSpeak/commit/01d57828db1f45491905b06fdaa40be93ba7f341))


### Styles

* **web:** 清理 Biome 警告（未使用 catch 绑定 / 非空断言 / forEach 返回值） ([93b982e](https://github.com/NoelOrin/GOSpeak/commit/93b982e1e37bf4933483bcd04081cad4f53b6dfc))
* 清理遗留 Biome 警告（ForgotPasswordModal / kvStore.test） ([948850c](https://github.com/NoelOrin/GOSpeak/commit/948850c5c8efde404d3fadd060182eed5f763e1c))
* 清零剩余 Biome 警告（未使用参数 / 可选链） ([9513996](https://github.com/NoelOrin/GOSpeak/commit/9513996e02f76e8d71ad23ba68f80a33765616e4))


### Tests

* **auth:** sync login failure contract to 401 ([9996589](https://github.com/NoelOrin/GOSpeak/commit/9996589720f245f77d9db0c5246397369067f527))
* cover per-domain role management flow ([e056ee5](https://github.com/NoelOrin/GOSpeak/commit/e056ee58e7b9dca24d64a8d0119ee10647e85054))
* **docs:** add unit tests and domain room management plan ([8791bd3](https://github.com/NoelOrin/GOSpeak/commit/8791bd3edb37109cc0285b60b056874a0cbe592f))
* **e2e:** adapt room helpers to new APIs and form ids ([7c938e2](https://github.com/NoelOrin/GOSpeak/commit/7c938e2598d1b7e817ffe586bf308a68a852dbe5))
* **e2e:** add guild, ws, and cleanup helper scripts ([3509a0f](https://github.com/NoelOrin/GOSpeak/commit/3509a0fbb8619958943a1af4cd9eb0d18f963c40))
* **guild:** add full-layer Guild tests ([bc51d4c](https://github.com/NoelOrin/GOSpeak/commit/bc51d4c3669e99a9cfa6a1ebe2d838a1d95f60c9))
* **integration:** add vitest API integration suite ([32ca2c7](https://github.com/NoelOrin/GOSpeak/commit/32ca2c7f7dae1aeb48e51984ade0fb6960b5d52e))
* **message:** 覆盖广播 DTO 的 author 富字段 ([47097eb](https://github.com/NoelOrin/GOSpeak/commit/47097eb7f911e16aee4f3b5297d5111dda157396))
* **room:** adapt fixtures to domain permission checks ([7e49e3d](https://github.com/NoelOrin/GOSpeak/commit/7e49e3da9a9ca9fd47285f7465c8dd87e40d99b0))
* **server:** add room repo, routes, and hub filter tests ([a780ab3](https://github.com/NoelOrin/GOSpeak/commit/a780ab3742e15466a946863e124869aedad2b545))
* **server:** align cloudflare provider tests with session metadata ([461b8f0](https://github.com/NoelOrin/GOSpeak/commit/461b8f010abfc1f7360219f2f5fd0ef55d27b1cb))
* **server:** fix guild handler mock JWT context and add WS upgrader e2e ([3cd821a](https://github.com/NoelOrin/GOSpeak/commit/3cd821aa23c864ee84eacfeef9da7782b91b0d2d))
* **signal:** add cluster signal handler tests and improve signal handler ([e003b2d](https://github.com/NoelOrin/GOSpeak/commit/e003b2df98d0ca807e859829b4df96b9fd705e13))
* **signal:** migrate tests to ws.ClientMessenger and mockBroadcaster ([0b43fa8](https://github.com/NoelOrin/GOSpeak/commit/0b43fa847bc8f32d22ec0b52ca489ef18cdbd439))
* **web:** add Guild API, store, and component tests ([ea450f7](https://github.com/NoelOrin/GOSpeak/commit/ea450f788e47bdf02476d5e5d8d5f4622c472240))
* **ws:** add protocol test and hub-guild WS integration test ([98789dd](https://github.com/NoelOrin/GOSpeak/commit/98789dda7523c5e7444ea26eee47ad260869b341))
* **ws:** add WebSocket benchmark test ([6f3f566](https://github.com/NoelOrin/GOSpeak/commit/6f3f566450b792d93a5466119081b9bbe4b8f412))
* **ws:** add WS package tests and testutil ([45f3c6b](https://github.com/NoelOrin/GOSpeak/commit/45f3c6bd3badf5d202fbb24a1853da38b89af64a))
* **ws:** fix concurrent test, ack validation, and guild test cleanup ([756a373](https://github.com/NoelOrin/GOSpeak/commit/756a3737a14f63919debb8f8aba989b3182790aa))


### Chores

* **agents:** 新增 room-voice-e2e 技能 ([8fcad69](https://github.com/NoelOrin/GOSpeak/commit/8fcad6967c08a9c2c9be7f243d8107d8af317040))
* **assets:** add preset avatar SVGs for user profile ([5dbc70a](https://github.com/NoelOrin/GOSpeak/commit/5dbc70ae2a1c68fa5dbdddc45342f12641ba07a1))
* **claude:** add web-debug launch config ([f1ab616](https://github.com/NoelOrin/GOSpeak/commit/f1ab61612d1251932e82ff00a3377af2593f38ae))
* clean redundant artifacts and add unused code audit ([992982a](https://github.com/NoelOrin/GOSpeak/commit/992982aaca8c17d8d0e1f299550d2b6df64c04f6))
* **deps:** add qrcode and bot media runtime deps ([9902499](https://github.com/NoelOrin/GOSpeak/commit/99024997d8a7dc3673f82a37ecc3aceb45e294a1))
* **deps:** configure dependabot for npm/gomod/actions (Mon/Fri) ([8e7369a](https://github.com/NoelOrin/GOSpeak/commit/8e7369a45ea7ceaafe34b8a7531aa9c6d13e1d83))
* **docs:** cleanup completed superpowers plans ([75dccea](https://github.com/NoelOrin/GOSpeak/commit/75dccea2ebcfc9a4866e53cd87ec6c8b6a78e328))
* **docs:** remove stale/expired cluster & SFU plans ([e610bbe](https://github.com/NoelOrin/GOSpeak/commit/e610bbe91095727cacad291f62ef077775d37a92))
* **e2e:** update room voice helpers and test spec ([283dbe8](https://github.com/NoelOrin/GOSpeak/commit/283dbe8051c8385beb320cc50c0d937d8af01e40))
* **frontend:** UI improvements for sidebar, profile and title hook ([c56b364](https://github.com/NoelOrin/GOSpeak/commit/c56b3646a1898e912a0aab697645750756ccdd4a))
* **im:** drop obvious ListByRoom reverse comment ([d425e88](https://github.com/NoelOrin/GOSpeak/commit/d425e888684c4ac9225873487a671a493c8b0192))
* initial release ([13ddfc2](https://github.com/NoelOrin/GOSpeak/commit/13ddfc22b12ffe312acb6ff4c53f5fe54b57532a))
* merge auth session p0 fixes ([adf6efe](https://github.com/NoelOrin/GOSpeak/commit/adf6efe63b54612e0edd8f609d5d05ae95ffeab8))
* release 0.2.0 ([d465747](https://github.com/NoelOrin/GOSpeak/commit/d4657471ca5ef0f300477a3fb16f3a6c103813e0))
* release 0.2.1 ([22c06fc](https://github.com/NoelOrin/GOSpeak/commit/22c06fc360d3fdd01e6bbd80f304119253c6029f))
* release 0.2.2 ([d381dbf](https://github.com/NoelOrin/GOSpeak/commit/d381dbff4d75a22346b61ff5cc1f388868a8c635))
* release 0.2.3 ([1844def](https://github.com/NoelOrin/GOSpeak/commit/1844defeffc1172b6b4612c927e51cb952ad6659))
* remove go-socket.io dependency and gorilla websocket patch ([9cd2fc9](https://github.com/NoelOrin/GOSpeak/commit/9cd2fc9d131490fe9f39fb30d5ebe60ad9ddc16f))
* sync lockfile after dependency dedupe ([30f07e0](https://github.com/NoelOrin/GOSpeak/commit/30f07e0f00bb5c427d520b4b985a84e42266d9a1))
* update deployment docs, plans, agents and agent skills ([6b49f96](https://github.com/NoelOrin/GOSpeak/commit/6b49f967aace2aa224e900a655ca32b355b8d330))
* 升级 biome 2.5.1，修复全部 lint 问题 ([b813017](https://github.com/NoelOrin/GOSpeak/commit/b813017a205499eb491cdf18a07c50fa08554872))

## [0.3.0](https://github.com/NoelOrin/GOSpeak/compare/v0.2.3...v0.3.0) (2026-08-28)

> 自 v0.2.3 以来的发布，共 28 个变更点（feat 12 / fix 4 / refactor 3 / style 2 / perf 1 / docs 4 / ci 1 / chore 1），基于 1 个合并提交（PR #17，含 28 个源提交）。

### Features

* feat(guest): 游客访问权限体系（guest 登录、guard 中间件、配置/封禁/清理/续期 API、Domain 级 listen/speak/message 开关）
* feat(guest): GuestService join/ban 与 DomainGuestBan 模型 + 仓库
* feat(guest): 前端游客入口页、登录游客按钮、guest store 与 api client
* feat(guest): 按 Domain 能力门控的游客 UI 与管理员审核界面
* feat(guest): 在 signal/sfu/message 路径强制 listen/speak/message 开关
* feat(web): 登录页重设计（鼠标视差）
* feat(auth): 重构登录权限服务 + 邮箱验证码模板按主应用风格重写
* feat(permission): 新增基于 casbin 的 Domain 权限适配器

### Bug Fixes

* fix(guest): 加固 cleanup、join guard 与 renewal 校验
* fix(guest): 加固 ban check、guard 白名单与 speak-off 强制
* fix(guest): guest join 响应不泄露邀请码
* fix(cluster): 加固多节点 agent-worker 运行时

### Refactor

* refactor(storage): 将 MinIO 替换为 RustFS 对象存储
* refactor(auth): 抽取 issueTokens 令牌对辅助函数
* refactor(ws): 移除 Fanout.marshalCount 生产字段，测试改用收消息数断言

### Style

* style(web): 登录页重设计 + 鼠标视差
* style(server): gofmt guest handler 与 router imports

### Performance

* perf(sfu): DynamicProvider 配置缓存 + Fanout 反向索引

### Documentation

* docs(guest): 游客访问权限设计文档与实现计划
* docs(guest): 游客访问路由说明

### CI

* ci: 重写 release 工作流，支持 push 到分支时自动发布

## [0.2.3](https://github.com/NoelOrin/GOSpeak/compare/v0.2.2...v0.2.3) (2026-08-22)

> 自 v0.2.2 以来的发布，共 5 个变更点（refactor 1 / fix 2 / style 1 / ci 1），基于 1 个提交。

### Refactor

* refactor(auth): integrate casbin, authboss, go-oauth2 and go-mail

### Bug Fixes

* fix(server): harden signal, SFU, auth and infra edge cases
* fix(server): permanent bot NULL expiry, JWT compat, SFU cleaner lifecycle and hub heartbeat

### Style

* style(web): replace favicon/logo with new A2 mark

### CI

* ci: remove release-please workflow

## [0.2.2](https://github.com/NoelOrin/GOSpeak/compare/v0.2.1...v0.2.2) (2026-08-21)

> 自 v0.2.1 以来的发布，共 8 个变更点（refactor 1 / fix 2 / style 1)，基于 1 个提交。

### Refactor

* refactor(auth): integrate casbin, authboss, go-oauth2 and go-mail

### Bug Fixes

* fix(server): harden signal, SFU, auth and infra edge cases
* fix(server): permanent bot NULL expiry, JWT compat, SFU cleaner lifecycle and hub heartbeat

### Features

* feat(auth): use Authboss BCryptHasher for password hash/verify (compatible with existing bcrypt hashes)
* feat(oauth): migrate token exchange for GitHub/Google/QQ/Generic to golang.org/x/oauth2
* feat(email): replace manual SMTP with go-mail client (SSL 465 / STARTTLS 587)
* feat(web): add role permission management page with create/delete role and permission sync
* feat(permission): replace in-memory cache with Casbin SyncedEnforcer backed by role_permissions table

### Style

* style(web): replace favicon/logo with new A2 mark (acid green tile, black bubble, white G)

## [0.2.1](https://github.com/NoelOrin/GOSpeak/compare/v0.2.0...v0.2.1) (2026-08-19)

> 自 v0.2.0 以来的发布，共 12 个变更点（fix 10 / chore 1 / ci 1)，基于 1 个提交。

### Bug Fixes

* fix(audit): validation, AuditIP via RemoteIP, nil-DB guard, dropped counter, logger
* fix(handler): validate audit params, use AuditIP in domain/mute/room/user
* fix(domain): atomic ResetInviteCode with RETURNING fallback, reset_invite audit
* fix(domain/web): invite error handling, QR race and clipboard timer fixes
* fix(sfu): CachedMuteRuleStore L1 cache, none-backend warnings, Agora/SRS guards
* fix(sfu/srs): publish block error handling and SRS store wiring warning
* fix(bus): document NATS mandatory for membership/mute stores
* fix(server): fix duplicate plugin StopAll in graceful shutdown
* fix(web): link page rAF dialog and timer handling
* fix(bot): speakingRooms ordering, SFU token inside try, identity guard

### Chore

* chore: extend lefthook pre-push (biome ci, go vet, typecheck, go test)

### CI

* ci: build docker image and binaries on release published

## 0.2.0 (2026-08-18)

> 自 v0.1.0-alpha1 以来的发布，共 287 个提交（feat 129 / fix 65 / perf 1 / docs 20 / ci 3）。

### Features

* feat(bot): improve socket client capabilities and add tests
* feat(web): domain invite share modal and link page
* feat(sfu): consolidate hard mute rule store across sfu/bus and agora/srs providers
* feat(domain): harden domain RBAC, mute, room and user management with db migration
* feat(audit): add audit log module and API routes
* feat(sfu-client): event-driven speaking detection
* feat(sfu): hard mute fixes, provider hardening and UI contract alignment
* feat(sfu,srs): Discord-style hard mute and SRS API room management
* feat(handler): add domain-first resource permission helper
* feat: enhance message, auth, WebSocket and cluster capabilities
* feat: 域权限优先并下线全局权限页
* feat: cluster agent worker, room resolver and voice session updates
* feat: enforce domain permissions on message deletion
* feat: add domain role management APIs and enforce domain permissions
* feat: add domain role management UI
* feat: add per-domain role and permission service
* feat: migrate and seed per-domain roles
* feat: add domain role repository and seed
* feat: add frontend domain role API and permission cache
* feat: add per-domain role models
* feat(db): support Turso libSQL with auto migration and tests
* feat(sfu-client): implement restartIce in mediasoup client for transport reconnection
* feat: cluster agent runtime + observability stack + comprehensive test suite
* feat(web): pass explicit domain when creating room
* feat(web): room list type icons per room type
* feat(mediasoup-worker): 显式资源清理与单元测试
* feat(web): Socket 状态机、乐观消息幂等与断线清理
* feat(server): 注入 bot token 校验、禁言过期与域踢出接线
* feat(ws): 连接状态机与错误 ACK 脱敏
* feat(signal): 域内踢人与踢出冷却，join 注册移出全局锁
* feat(sfu): 能力矩阵覆盖、provider 资源释放与错误测试
* feat(mute): 临时禁言到期后完整解禁
* feat(message): 稳定作者 UUID、消息幂等与软删除状态
* feat(domain): 事务化归属转移与跨实例踢出命令
* feat(auth): 加固 JWT 鉴权与用户状态管理
* feat(bus): Redis 成员 CAS 与 JetStream 消息去重
* feat(signal): 多实例房间元数据与成员原子注册
* feat(bus): 强化共享状态存储与断线恢复
* feat(cluster): add leader lock and auto-scaling hooks
* feat(deploy): add cluster nginx routing
* feat(web): add cluster management page
* feat(cluster): expose cluster health stats
* feat(cluster): reconcile cluster state on agent startup
* feat(cluster): worker executes NATS control commands
* feat(cluster): publish control commands from agent
* feat(cluster): define NATS control command envelope
* feat(cluster): explicitly reject worker business writes
* feat(cluster): worker mode uses read-only DB and skips seeding
* feat(web): route voice signal join to assigned worker
* feat(web): support worker URL socket connection
* feat(domain): enrich member list with user names
* feat(frontend): update domain detail page with room improvements
* feat(room): add domain-required validation and update room module
* feat(skills): update room-voice-e2e skill documentation
* feat(frontend): update socket store and API
* feat(backend): update message service and signal hub
* feat(frontend): update UI components, pages and assets
* feat(frontend): update UI components, pages and stores
* feat(auth): add auth middleware improvements and user service enhancements
* feat(room): add room duplicate detection and list utilities
* feat(message): enhance message and conversation services with tests
* feat(oauth): add OAuth account encryption and provider models
* feat(domain): implement complete Domain module with CRUD and tests
* feat(cluster): add cluster events, scheduler tests and handler APIs
* feat: add cluster module, improve storage service, and polish frontend UI
* feat(domain): add room management to domain admin page
* feat(storage): allow PDF and plain text uploads
* feat(room): improve room list and detail components
* feat(user-group): add user group module (backend + frontend)
* feat(web): add domain room table
* feat(web): add edit room modal
* feat(web): support target domain in create room modal
* feat(web): add room list update delete api
* feat(server): enforce room manage permission on update and delete
* feat(web): polish room and domain UI
* feat(web): add guild invite preview and polish invite flow
* feat(web): add GuildMemberTable component and guild management page
* feat(web): update room list UI, guild index page, and room state management
* feat(web): update guild store and API with multi-guild support
* feat(signal): update hub with guild namespace isolation and state sync
* feat(server): update room handler and service with guild isolation
* feat(server): add guild middleware for request context and DB init improvements
* feat(bot): add media routing, TTS, ASR and WS ticket auth
* feat(web): add guild discovery, text chat and UX improvements
* feat(server): complete guild permissions, discovery and room isolation
* feat(server): harden auth, uploads and WebSocket handshake
* feat(ws): complete socket.io to WebSocket migration
* feat: socket
* feat(server): add guildUUID to JoinPolicy interface
* feat(server): reset-password CLI command and planning docs
* feat(web): private message API types and socket events
* feat(socket): add native WebSocket client adapter
* feat(guild): integrate GuildList into layout, add create/join buttons, enhance guild page
* feat(chat): add private chat UI with conversation list, chat window, member sidebar, and /chat route
* feat(socket): bind PRIVATE_NEW event globally in socketStore
* feat(signal): wire private:send WS event and /conversation/send endpoint
* feat(service): add SendDirect for private chat messages
* feat(model): add conversation fields to Message and private chat event constants
* feat: 添加文字聊天功能并完善相关文档与接口
* feat: compact
* feat(chat): add frontend conversation API, chat store, and IDB cache
* feat(chat): add DM signal events and personal room routing
* feat(chat): add DM service, handler, router, and DI wiring
* feat(chat): add direct message model and repository layer
* feat(signal): add OnGuildDelete cleanup and DRY test setup
* feat(web): extend PIP keepalive for AndroidTWA and improve error handling
* feat(android): add Android TWA scaffold
* feat(web): add PIP keepalive utility for iOS picture-in-picture
* feat(ws): add ws package (Client, Fanout, HandlerRegistry, Upgrader) + WSDeliverer
* feat(guild): add default Guild migration for existing rooms
* feat(guild): register Guild permissions in seed data
* feat(guild): add frontend Guild API, store, components, and route page
* feat(guild): namespace Signal Hub rooms by guild UUID
* feat(guild): add GuildUUID filtering to Room CRUD and signal roomStore interface
* feat(guild): add Guild handler, routes, middleware, and DI wiring
* feat(guild): add GuildService with create/join/leave/kick/transfer
* feat(guild): add GuildRepository with CRUD and member operations
* feat(guild): add GuildUUID to Room/Message models and guild permcodes
* feat(guild): add Guild and GuildMember data models
* feat(service): add async write-steal pattern to message service
* feat(signal): add KV membership fallback to message bridge
* feat(signal): add ByIdentity index for O(1) member lookup in Hub
* feat(bus): add ConcurrentDeliverer with concurrent fanout to all SIO clients
* feat(im): message list API and DI wiring
* feat(im): socket message:send bridge via MessageService
* feat(im): message service with EventBus publish
* feat(im): add message model and repository
* feat(sfu): 管理页按 provider 隔离保存配置
* feat(web): APIKey 吊销改用确认弹窗

### Bug Fixes

* fix(signal): deny ws message access without claims
* fix(signal): fail closed when ws domain permission checker missing
* fix(signal): enforce domain role permissions on ws message events
* fix(domain): restrict admin assignment to domain owner
* fix(handler): apply claims-aware checks to room manage and delete others
* fix(handler): honor explicit claims permissions on platform fallback
* fix(message): enforce domain role permissions on message endpoints
* fix(room): enforce resource permissions in handler for all room routes
* fix(room): let domain roles manage rooms without global permission gate
* fix(handler): guard domain permission helper against typed nil checkers
* fix(room): enforce domain role permissions on room endpoints
* fix(auth): revoke HttpOnly refresh cookie on logout
* fix(auth): rotate refresh family on every refresh
* fix: set cookie Secure flag based on TLS to avoid cookie issues in prod
* fix: heartbeat goroutine exit wait + S3 5s timeout context
* fix: mediasoup restart-ice, form type safety, domain_uuid validation, permissions from profile
* fix(sfu-client): match domain-scoped room events
* fix(bus): sanitize colons in NATS KV keys
* fix(server): require domain on room create and broadcast room list
* fix(web): room list scrolling with hidden scrollbar
* fix: review gap fixes
* fix(handlers): 控制面失败不阻塞业务响应并拒绝 SRS 非法回调
* fix(plugin): 拒绝非法生命周期状态迁移
* fix(ws): 连接级唯一 ID、心跳保活与幂等关闭
* fix: 修复跨域房间切换与 chatStore 循环依赖
* fix: harden cluster/signal/web/deploy review findings
* fix(server): repair upload ownership check and plugin secret encryption
* fix(server): harden oauth bot plugin auth and secret storage
* fix(server): harden ws lifecycle and speaking mute checks
* fix(server): harden cluster lifecycle, key rotation and state sync
* fix(server): bind cloudflare sessions to their owner
* fix(server): verify webhook signatures and honor bot claim permissions
* fix(server): keep permanent mutes enforced beyond 24 hours
* fix(server): reload storage config and enforce upload ownership
* fix(web): pass domain_uuid and sfuRoom through voice join
* fix(server): read domain_uuid from middleware context
* fix(web): sync frontend permissions and update AGENTS docs
* fix(signal): use composite room keys for media cleanup and slots
* fix(server): enforce domain membership and use composite sfu room
* fix(router): normalize import ordering and fix test coverage
* fix(text-room): improve message input and rendering
* fix(room): remove unused voiceChat component
* fix(web): surface edit room errors and unify validation
* fix(web): correct edit room modal import casing
* fix: remove duplicate ConversationList import + add @tanstack/solid-virtual
* fix(packages): bot permissions, mediasoup dedup, srs cleanup
* fix(web): form validator, UserInfo type, and vite config guard
* fix(server): concurrency safety for role cache and job queue
* fix(server): path traversal protection and nil pointer fixes
* fix(guild): add error handling, loading text, and data-ready guard to guild page
* fix(chat): set author_id on optimistic messages and remove duplicate PM listeners
* fix(handler): monitor 与 mute 处理器边界条件修复
* fix(plugin): TOCTOU 竞争修复与注册逻辑完善
* fix(service): 错误处理统一与边界检查完善
* fix: compilation blockers, panic risks, crypto, SFU provider bugs
* fix(im): 修未读数bug + fanout nil保护 + 消息竞态 + ack/事务/nit
* fix(android): robust startForeground with try/catch and logging
* fix(server): race-safe guild checker, stack-allocated invite code, testutil options
* fix(message): add write-worker fallback and graceful shutdown
* fix(bot): add livekit-client type declaration for optional dependency
* fix(im): address review findings
* fix(ops,signal,web): cherry-pick portable fixes from mobile-responsive branch
* fix(web): 重连成功后关闭 reconnecting toast
* fix(auth): 开发态固定 JWT 静态密钥
* fix: 小问题

### Performance

* perf(signal): batch localRoomSnapshots to eliminate N+1 in getMergedRoomsScoped

### Documentation

* docs: mark cluster-agent-worker plan as delivered with verification evidence
* docs: add bilingual changelog baseline
* docs(review): split p0 auth and domain rbac plans
* docs(review): mark repo hygiene items as handled
* docs: sync review status and gap-fix plan completion
* docs: 同步 Domain 术语与多实例状态同步说明
* docs(cluster): mark control plane completion status
* docs: add tauri mobile hybrid webrtc design and plan
* docs: update AGENTS.md and documentation
* docs: update deployment docs and superpowers specs/plans
* docs(config): update AGENTS.md, vite config and frontend documentation
* docs: update AGENTS and swagger for domain rename
* docs: design domain room management feature
* docs(web): define room and domain UI
* docs: update AGENTS.md and regenerate swagger docs
* docs: expand cluster agent worker plan with discovery phase
* docs: update AGENTS.md across monorepo for new modules
* docs(guild): update AGENTS.md with Guild architecture and route table
* docs: add websocket fanout migration plan
* docs(plans): room IM over EventBus implementation plan

### CI/CD

* ci: start releases from v0.2.0
* ci: merge release-please automated tagging
* ci: add release-please automated tagging

## v0.1.0-alpha1 (2026-07-15)

- 历史基线：首个 alpha 发布 / Historical baseline: initial alpha release.
