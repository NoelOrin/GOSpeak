import { createFileRoute, useNavigate } from "@tanstack/solid-router";
import { createMemo, createResource, createSignal, Show } from "solid-js";
import { joinDomain, previewDomainInvite } from "@/api/domain";
import DomainInvitePreview from "@/components/domain/DomainInvitePreview";
import domainStore from "@/stores/domainStore";
import { extractDomainInviteCode } from "@/utils/domainInvite";

export const Route = createFileRoute("/(app)/invite/d/$code/")({
	component: RouteComponent,
	staticData: {
		title: "邀请加入",
		icon: "link",
	},
});

function RouteComponent() {
	const params = Route.useParams();
	const navigate = useNavigate();
	const inviteCode = createMemo(() => extractDomainInviteCode(params().code));
	const [domain] = createResource(inviteCode, (code) => {
		if (!code) throw new Error("invalid invite code");
		return previewDomainInvite(code);
	});
	const [joining, setJoining] = createSignal(false);
	const [error, setError] = createSignal("");

	const joined = createMemo(() => {
		const current = domain();
		return !!current && domainStore.state.myDomainUUIDs.includes(current.uuid);
	});

	async function handleJoin() {
		const current = domain();
		if (!current) return;
		if (joining()) return;
		if (joined()) {
			navigate({
				to: "/domain/$domainUUID",
				params: { domainUUID: current.uuid },
			});
			return;
		}
		setJoining(true);
		setError("");
		try {
			const joinedDomain = await joinDomain(current.invite_code);
			domainStore.addDomain(joinedDomain);
			domainStore.setCurrentDomain(joinedDomain.uuid);
			navigate({
				to: "/domain/$domainUUID",
				params: { domainUUID: joinedDomain.uuid },
			});
		} catch (e: any) {
			setError(e?.response?.data?.msg || "加入失败");
		} finally {
			setJoining(false);
		}
	}

	return (
		<div class="flex-1 flex items-center justify-center p-4 overflow-y-auto">
			<div class="card w-full max-w-md bg-base-100 border border-base-300">
				<div class="card-body">
					<Show
						when={inviteCode()}
						fallback={
							<div class="alert alert-error">
								<span>邀请链接无效或域不存在</span>
							</div>
						}
					>
						<DomainInvitePreview
							domain={domain()}
							joined={joined()}
							loading={domain.loading}
							error={domain.error ? "邀请链接无效或域不存在" : error()}
							joining={joining()}
							onConfirm={handleJoin}
						/>
					</Show>
				</div>
			</div>
		</div>
	);
}

export default RouteComponent;
