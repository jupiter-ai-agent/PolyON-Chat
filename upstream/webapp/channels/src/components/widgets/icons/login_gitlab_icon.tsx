// Copyright (c) 2026 Triangle.s — PolyON Platform
// PolyON SSO Icon (replaces GitLab icon — Keycloak OIDC)

import React from 'react';

export default function LoginGitlabIcon(props: React.HTMLAttributes<HTMLSpanElement>) {
    return (
        <span {...props}>
            <svg
                width='17'
                height='17'
                viewBox='0 0 17 17'
                fill='none'
                xmlns='http://www.w3.org/2000/svg'
                aria-label='PolyON SSO'
            >
                {/* 열쇠 모양 — SSO/인증 의미 */}
                <circle cx='6.5' cy='7' r='4' stroke='currentColor' strokeWidth='1.8' fill='none'/>
                <circle cx='6.5' cy='7' r='1.8' fill='currentColor'/>
                <rect x='9.5' y='6.2' width='6' height='1.6' rx='0.8' fill='currentColor'/>
                <rect x='13' y='7.8' width='1.6' height='2' rx='0.8' fill='currentColor'/>
                <rect x='11' y='7.8' width='1.6' height='1.6' rx='0.8' fill='currentColor'/>
            </svg>
        </span>
    );
}
