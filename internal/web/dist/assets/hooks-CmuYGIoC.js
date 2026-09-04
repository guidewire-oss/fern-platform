import{j as o,H as u,l as n,I as i}from"./index-Bgsf7zuL.js";import{u as c}from"./Button-DvaeUvP1.js";/**
 * @license lucide-react v0.400.0 - ISC
 *
 * This source code is licensed under the ISC license.
 * See the LICENSE file in the root directory of this source tree.
 */const p=o("Star",[["polygon",{points:"12 2 15.09 8.26 22 9.27 17 14.14 18.18 21.02 12 17.77 5.82 21.02 7 14.14 2 9.27 8.91 8.26 12 2",key:"8f66p6"}]]),f=`
  query MyPrefs {
    userPreferences {
      theme
      timezone
      language
      favorites
    }
  }
`,y=`
  mutation Toggle($projectId: String!) {
    toggleProjectFavorite(projectId: $projectId) {
      favorites
    }
  }
`,t=["user-preferences"];function m(){return u({queryKey:t,queryFn:()=>n(f),staleTime:6e4})}function v(){const a=i();return c({mutationFn:s=>n(y,{projectId:s}),onMutate:async s=>{await a.cancelQueries({queryKey:t});const r=a.getQueryData(t);if(r!=null&&r.userPreferences){const e=new Set(r.userPreferences.favorites);e.has(s)?e.delete(s):e.add(s),a.setQueryData(t,{userPreferences:{...r.userPreferences,favorites:Array.from(e)}})}return{previous:r}},onError:(s,r,e)=>{e!=null&&e.previous&&a.setQueryData(t,e.previous)},onSettled:()=>a.invalidateQueries({queryKey:t})})}export{p as S,m as a,v as u};
//# sourceMappingURL=hooks-CmuYGIoC.js.map
