- Keep base and mod in int64, so that intermediate multiplication doesn't cross 64 boundary.
- Use base = 10^5+7 and mod = 10^12+7 to avoid hash collisions
- Add extra mod when subtracting in mod operations to avoid negative value.
- Use second mid point in binary search and break like s to m-1 and m to e.
  
```
class Solution {
public:

    // keep base and mod in int64, so that intermediate multiplication doesn't cross 64 boundary
    int64_t base = 100007; //10^5 + 7
    int64_t mod = 1000000000007LL; //10^12 + 7
    /* This causes hash collisions
       int64_t base = 31;
       int64_t mod = 1000000000007LL; 
    */


    bool checkCommanPathExists(vector<vector<int>>& paths, int w, vector<int64_t>&pow){
        unordered_map<int64_t, int> count;
        for(auto& path: paths){
            unordered_set<int64_t> st;
            if(path.size() < w) return false;
            
            //compute rolling hash of first window w elements
            int i=0, j=w-1;
            int64_t hash = path[i];
            for(int k=i+1;k<=j;k++)
                hash = ((hash * base) % mod + path[k]) % mod;

            if(st.find(hash) == st.end())
                count[hash]++;
            st.insert(hash);

            i++;j++;
            while(j<path.size()){
                // roll hash to remove i-1th and add jth element
                // when subtracting with mod operation can result in negative number, so add mod again to make it positive
                hash = (hash - (path[i-1] * pow[w-1]) % mod + mod) % mod;
                hash = (hash * base) % mod;
                hash = (hash + path[j]) % mod;

                if(st.find(hash) == st.end())
                    count[hash]++;
                st.insert(hash);

                i++;j++;
            }
        }
        for (auto& [h, cnt] : count) {
            if (cnt == paths.size()) {
                return true;
            }
        }
        for(auto&itr : count)
            if(itr.second == paths.size())
                return true;
        return false;
    }

    int longestCommonSubpath(int n, vector<vector<int>>& paths) {
        int max_sz = paths[0].size();
        for(auto& path : paths) max_sz = std::max(max_sz, (int)path.size());

        vector<int64_t> pow;
        pow.resize(max_sz + 1);
        pow[0] = 1; pow[1] = base;
        for(int i=2; i < max_sz+1; i++) 
            pow[i] = (pow[i-1] * base) % mod;

        int s=1, e=sz;
        while(e-s > 0){
            int m = (s+e+1)/2; //use second mid point to break s-m-1 and m to e
            if(checkCommanPathExists(paths, m, pow) == false)
                e = m-1;
            else
                s = m;
        }
        
        if(checkCommanPathExists(paths, s, pow))
            return s;
        return 0;
    }
};
```
