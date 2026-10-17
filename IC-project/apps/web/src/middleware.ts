import { NextResponse, type NextRequest } from "next/server";

// First line of defence only: it checks that a session cookie exists. The pages verify the session with the API.
export function middleware(req: NextRequest) {
  if (req.cookies.has("ic_session") || req.cookies.has("__Host-ic_session")) return NextResponse.next();
  const url = req.nextUrl.clone();
  url.pathname = "/login";
  url.search = "";
  url.searchParams.set("next", req.nextUrl.pathname);
  return NextResponse.redirect(url);
}

export const config = { matcher: ["/admin/:path*", "/portal/:path*", "/change-password"] };
