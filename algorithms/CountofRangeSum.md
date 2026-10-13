- Use lower_bound (ge) and upper bound (g) on sorted vector only in O(logn).
- Use std::distance to get distance between iterator in O(1);
- Use vec.begin()+s and vec.begin()+e correctly to confine properly within range of merge sort.
- Correct indices for sorted and presum array.
- In our merge sort, we don't count for full prefix from 0th, so count them separately. Our merge sort takes diff of two indices.
```
class Solution {
public:


    int64_t mergeSortExt(vector<int>& nums, vector<int64_t>& presum, int64_t s, int64_t e, int lower, int upper){
        if(s == e)
            return 0;
        int64_t m = (s+e+1)/2;
        int64_t left_ans = mergeSortExt(nums, presum, s, m-1, lower, upper);
        int64_t right_ans = mergeSortExt(nums, presum, m, e, lower, upper);
        // both left and right are sorted now
        int64_t ans = left_ans + right_ans;
        for(int64_t i=m; i<=e; i++){
            int64_t ele = presum[i];
            int64_t right_max = ele - lower; // ele - right_max >= lower
            int64_t left_max = ele - upper; // ele - left_max <= upper
            //count number of elements between left_max and right max
            auto itru = std::upper_bound(presum.begin()+s, presum.begin() + m, right_max);
            auto itrd = std::lower_bound(presum.begin()+s, presum.begin() + m, left_max);
            ans += std::distance(itrd, itru);
        }

        //sort both
        vector<int64_t> sorted;
        int64_t i=s, j=m;
        while(i<=m-1 or j<=e){
            if(i > m-1)
                sorted.push_back(presum[j++]);
            else if(j > e)
                sorted.push_back(presum[i++]);
            else if(presum[i] <= presum[j])
                sorted.push_back(presum[i++]);
            else
                sorted.push_back(presum[j++]);
        }
        for(int i=s; i<= e; i++)
            presum[i] = sorted[i-s];
        return ans;
    }

    int countRangeSum(vector<int>& nums, int lower, int upper) {
        vector<int64_t> presum;
        presum.push_back(nums[0]);
        int64_t ans = 0;
        if(presum[0]>=lower and presum[0]<=upper)
                ans++;
        for(int i=1; i<nums.size(); i++){
            presum.push_back(presum[i-1] + nums[i]);
            if(presum[i]>=lower and presum[i]<=upper)
                ans++;
        }            
        return ans + mergeSortExt(nums, presum, 0, presum.size() - 1, lower, upper);
    }
};
```
