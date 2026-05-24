import Nav from "@/layouts/common/nav";

const Layout = ({ children }: { children: JSX.Element }) => {
	return (
		<div class="flex h-full">
			<Nav />
			<div class="flex-1">{children}</div>
		</div>
	);
};
export default Layout;
